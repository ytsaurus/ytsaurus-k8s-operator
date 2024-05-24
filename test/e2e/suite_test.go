/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	ptr "k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	otypes "github.com/onsi/gomega/types"

	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var ctx context.Context
var log = logf.Log

func TestAPIs(t *testing.T) {
	if os.Getenv("YTSAURUS_ENABLE_E2E_TESTS") != "true" {
		t.Skip("skipping E2E tests: set YTSAURUS_ENABLE_E2E_TESTS environment variable to 'true'")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = SynchronizedBeforeSuite(func(ctx context.Context) []byte {
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)
	logf.IntoContext(ctx, logger)

	By("bootstrapping test environment")
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:        []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:    true,
		UseExistingCluster:       ptr.Bool(true),
		AttachControlPlaneOutput: true,
		Config:                   cfg,
	}

	_, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(testEnv.Stop)

	// Cannot serialize rest config here - just load again in each process and check host to be sure.
	return []byte(cfg.Host)
}, func(ctx context.Context, host []byte) {
	logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)
	logf.IntoContext(ctx, logger)

	By("bootstrapping k8s client")

	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg.Host).To(Equal(string(host)))

	err = ytv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = BeforeEach(func() {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.TODO())
	DeferCleanup(func() {
		cancel()
		ctx = nil
	})
})

func NewYtsaurusStatusTracker() func(*ytv1.Ytsaurus) bool {
	prevStatus := ytv1.YtsaurusStatus{}
	conditions := map[string]metav1.Condition{}
	updateConditions := map[string]metav1.Condition{}

	return func(ytsaurus *ytv1.Ytsaurus) bool {
		if ytsaurus == nil {
			return false
		}

		changed := false
		newStatus := ytsaurus.Status
		if prevStatus.State != newStatus.State {
			log.Info("ClusterStatus", "state", newStatus.State)
			changed = true
		}

		for _, cond := range newStatus.Conditions {
			if prevCond, found := conditions[cond.Type]; !found || !reflect.DeepEqual(cond, prevCond) {
				log.Info("ClusterCondition", "type", cond.Type, "status", cond.Status, "reason", cond.Reason, "message", cond.Message)
				changed = true
				conditions[cond.Type] = cond
			}
		}

		if prevStatus.UpdateStatus.State != newStatus.UpdateStatus.State {
			log.Info("UpdateStatus", "state", newStatus.UpdateStatus.State)
			changed = true
		}

		if len(prevStatus.UpdateStatus.Components) != len(newStatus.UpdateStatus.Components) {
			log.Info("UpdateStatus", "components", newStatus.UpdateStatus.Components)
			changed = true
		}

		if prevStatus.UpdateStatus.Flow != newStatus.UpdateStatus.Flow {
			log.Info("UpdateStatus", "flow", newStatus.UpdateStatus.Flow)
			changed = true
		}

		for _, cond := range newStatus.UpdateStatus.Conditions {
			if prevCond, found := updateConditions[cond.Type]; !found || !reflect.DeepEqual(cond, prevCond) {
				log.Info("UpdateCondition", "type", cond.Type, "status", cond.Status, "reason", cond.Reason, "message", cond.Message)
				changed = true
				updateConditions[cond.Type] = cond
			}
		}

		prevStatus = newStatus
		return changed
	}
}

func EventuallyYtsaurus(ctx context.Context, name types.NamespacedName, timeout time.Duration) AsyncAssertion {
	var ytsaurus ytv1.Ytsaurus
	trackStatus := NewYtsaurusStatusTracker()
	return Eventually(ctx, func(ctx context.Context) (*ytv1.Ytsaurus, error) {
		err := k8sClient.Get(ctx, name, &ytsaurus)
		if err == nil {
			trackStatus(&ytsaurus)
		}
		return &ytsaurus, err
	}, timeout, pollInterval)
}

func ConsistentlyYtsaurus(ctx context.Context, name types.NamespacedName, timeout time.Duration) AsyncAssertion {
	var ytsaurus ytv1.Ytsaurus
	trackStatus := NewYtsaurusStatusTracker()
	return Consistently(ctx, func(ctx context.Context) (*ytv1.Ytsaurus, error) {
		err := k8sClient.Get(ctx, name, &ytsaurus)
		if err == nil {
			trackStatus(&ytsaurus)
		}
		return &ytsaurus, err
	}, timeout, pollInterval)
}

func HaveClusterState(state ytv1.ClusterState) otypes.GomegaMatcher {
	return HaveField("Status.State", state)
}

func HaveClusterUpdateState(updateState ytv1.UpdateState) otypes.GomegaMatcher {
	return And(
		HaveClusterState(ytv1.ClusterStateUpdating),
		HaveField("Status.UpdateStatus.State", updateState),
	)
}

func HaveClusterUpdatingComponents(components ...string) otypes.GomegaMatcher {
	return And(
		HaveClusterState(ytv1.ClusterStateUpdating),
		HaveField("Status.UpdateStatus.Components", components),
	)
}
