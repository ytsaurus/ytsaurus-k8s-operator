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

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	gtypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	otypes "github.com/onsi/gomega/types"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clientgoretry "k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.WithWatch
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
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:        []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:    true,
		UseExistingCluster:       ptr.To(true),
		AttachControlPlaneOutput: true,
		Config:                   cfg,
	}

	_, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(testEnv.Stop)

	// Cannot serialize rest config here - just load again in each process and check host to be sure.
	return []byte(cfg.Host)
}, func(ctx context.Context, host []byte) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping k8s client")

	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg.Host).To(Equal(string(host)))

	scheme := runtime.NewScheme()

	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ytv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.NewWithWatch(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

func ShouldPreserveArtifacts() bool {
	suiteConfig, _ := GinkgoConfiguration()
	if !suiteConfig.FailFast {
		return false
	}
	return CurrentSpecReport().State.Is(gtypes.SpecStateFailed | gtypes.SpecStateTimedout)
}

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
	var generaion int64

	return func(ytsaurus *ytv1.Ytsaurus) bool {
		if ytsaurus == nil {
			return false
		}

		changed := false
		newStatus := ytsaurus.Status

		if ytsaurus.Generation != generaion {
			log.Info("Generation", "current", ytsaurus.Generation, "previous", generaion)
			generaion = ytsaurus.Generation
			changed = true
		}

		if prevStatus.ObservedGeneration != newStatus.ObservedGeneration {
			log.Info("ObservedGeneration", "current", newStatus.ObservedGeneration, "previous", prevStatus.ObservedGeneration)
			changed = true
		}

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

func GetObjectGVK(object client.Object) schema.GroupVersionKind {
	gvks, _, err := k8sClient.Scheme().ObjectKinds(object)
	Expect(err).ToNot(HaveOccurred())
	return gvks[0]
}

func CurrentlyObject[T client.Object](ctx context.Context, object T) Assertion {
	key := client.ObjectKeyFromObject(object)
	err := k8sClient.Get(ctx, key, object)
	Expect(err).ToNot(HaveOccurred())
	return Expect(object)
}

func EventuallyObject[T client.Object](ctx context.Context, object T, timeout time.Duration) AsyncAssertion {
	key := client.ObjectKeyFromObject(object)
	return Eventually(ctx, func(ctx context.Context) (T, error) {
		err := k8sClient.Get(ctx, key, object)
		return object, err
	}, timeout, pollInterval)
}

func UpdateObject(ctx context.Context, object client.Object) {
	current := object.DeepCopyObject().(client.Object)
	key := client.ObjectKeyFromObject(object)
	err := clientgoretry.RetryOnConflict(clientgoretry.DefaultRetry, func() error {
		Expect(k8sClient.Get(ctx, key, current)).To(Succeed())
		// Fetch current resource version: any status update changes it too.
		object.SetResourceVersion(current.GetResourceVersion())
		return k8sClient.Update(ctx, object)
	})
	Expect(err).ToNot(HaveOccurred())
}

func EventuallyYtsaurus(ctx context.Context, ytsaurus *ytv1.Ytsaurus, timeout time.Duration) AsyncAssertion {
	trackStatus := NewYtsaurusStatusTracker()
	name := client.ObjectKeyFromObject(ytsaurus)
	return Eventually(ctx, func(ctx context.Context) (*ytv1.Ytsaurus, error) {
		err := k8sClient.Get(ctx, name, ytsaurus)
		if err == nil {
			trackStatus(ytsaurus)
		}
		return ytsaurus, err
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

func HaveObservedGeneration() otypes.GomegaMatcher {
	var generation int64
	return SatisfyAll(
		HaveField("Generation",
			Satisfy(func(gen int64) bool {
				generation = gen
				return true
			})),
		HaveField("Status.ObservedGeneration",
			Satisfy(func(gen int64) bool {
				return gen == generation
			})),
	)
}

func HaveClusterState(state ytv1.ClusterState) otypes.GomegaMatcher {
	return HaveField("Status.State", state)
}

func HaveClusterStateRunning() otypes.GomegaMatcher {
	return HaveClusterState(ytv1.ClusterStateRunning)
}

func HaveClusterStateUpdating() otypes.GomegaMatcher {
	return HaveClusterState(ytv1.ClusterStateUpdating)
}

func HaveClusterUpdateFlow(flow ytv1.UpdateFlow) otypes.GomegaMatcher {
	return And(
		HaveClusterStateUpdating(),
		HaveField("Status.UpdateStatus.Flow", flow),
	)
	// FIXME(khlebnikov): Flow initial state is None which is weird.
}

func HaveClusterUpdateState(updateState ytv1.UpdateState) otypes.GomegaMatcher {
	return And(
		HaveClusterStateUpdating(),
		HaveField("Status.UpdateStatus.Flow", Not(Equal(ytv1.UpdateFlowNone))),
		HaveField("Status.UpdateStatus.State", updateState),
	)
}

func HaveClusterUpdatingComponents(components ...string) otypes.GomegaMatcher {
	return And(
		HaveClusterStateUpdating(),
		HaveField("Status.UpdateStatus.Flow", Not(Equal(ytv1.UpdateFlowNone))),
		HaveField("Status.UpdateStatus.Components", components),
	)
	// FIXME(khlebnikov): Flow initial state is None which is weird.
}

func HaveRemoteNodeReleaseStatusRunning() otypes.GomegaMatcher {
	return HaveField("Status.ReleaseStatus", ytv1.RemoteNodeReleaseStatusRunning)
}
