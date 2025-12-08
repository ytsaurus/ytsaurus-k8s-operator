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
	"flag"
	"os"
	"reflect"
	"testing"
	"time"

	"k8s.io/utils/ptr"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	logy "go.ytsaurus.tech/library/go/core/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/onsi/ginkgo/v2"
	gtypes "github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	oformat "github.com/onsi/gomega/format"
	otypes "github.com/onsi/gomega/types"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clientgoretry "k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.WithWatch
var clientset *kubernetes.Clientset

var specCtx context.Context
var log = logf.Log
var ytLogger *logy.Logger

func getDefaultE2EEnabled() bool {
	return os.Getenv("YTOP_ENABLE_E2E") == "true"
}

var enableE2E = flag.Bool("enable-e2e", getDefaultE2EEnabled(), "Enable e2e tests (can also be set via YTOP_ENABLE_E2E environment variable)")

var _ = BeforeEach(func() {
	if !*enableE2E {
		Skip("skipping E2E tests: add option --enable-e2e or set YTOP_ENABLE_E2E=true")
	}
})

func TestE2E(t *testing.T) {
	oformat.MaxLength = 20_000 // Do not truncate large YT errors
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Tests")
}

func setLogger() {
	logger := zap.New(
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(
				zap.NewDevelopmentEncoderConfig(),
			),
			zapcore.AddSync(GinkgoWriter),
			zap.DebugLevel,
		),
	)
	logf.SetLogger(zapr.NewLogger(logger))
	ytLogger = &logy.Logger{
		L: logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel)),
	}
}

var _ = SynchronizedBeforeSuite(func(ctx context.Context) []byte {
	setLogger()

	if !*enableE2E {
		return nil
	}

	By("bootstrapping test environment")
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	testEnv := &envtest.Environment{
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
	setLogger()

	if !*enableE2E {
		return
	}

	By("bootstrapping k8s client")

	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg.Host).To(Equal(string(host)))

	scheme := runtime.NewScheme()

	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ytv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = certmanagerv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.NewWithWatch(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	clientset, err = kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(clientset).NotTo(BeNil())
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
	Expect(specCtx).To(BeNil())
	specCtx, cancel = context.WithCancel(context.Background())
	DeferCleanup(func() {
		cancel()
		Expect(specCtx).ToNot(BeNil())
		specCtx = nil
	})
})

func LogObjectEvents(ctx context.Context, namespace string) func() {
	watcher, err := k8sClient.Watch(ctx, &corev1.EventList{}, &client.ListOptions{
		Namespace: namespace,
	})
	Expect(err).ToNot(HaveOccurred())

	logEvent := func(event *corev1.Event) {
		log.Info("Event",
			"type", event.Type,
			"kind", event.InvolvedObject.Kind,
			"name", event.InvolvedObject.Name,
			"reason", event.Reason,
			"message", event.Message,
		)
	}

	go func() {
		for ev := range watcher.ResultChan() {
			switch ev.Type {
			case watch.Added, watch.Modified:
				if event, ok := ev.Object.(*corev1.Event); ok {
					logEvent(event)
				}
			}
		}
	}()

	return watcher.Stop
}

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
			log.Info("ObservedGeneration",
				"current", newStatus.ObservedGeneration,
				"previous", prevStatus.ObservedGeneration,
				"observed", newStatus.ObservedGeneration == ytsaurus.Generation)
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

		if prevStatus.UpdateStatus.Flow != newStatus.UpdateStatus.Flow {
			log.Info("UpdateStatus", "flow", newStatus.UpdateStatus.Flow)
			changed = true
		}

		if len(prevStatus.UpdateStatus.UpdatingComponents) != len(newStatus.UpdateStatus.UpdatingComponents) {
			log.Info("UpdateStatus", "updatingComponents", newStatus.UpdateStatus.UpdatingComponents)
			changed = true
		}

		if len(prevStatus.UpdateStatus.UpdatingComponentsSummary) != len(newStatus.UpdateStatus.UpdatingComponentsSummary) {
			log.Info("UpdateStatus", "updatingComponentsSummary", newStatus.UpdateStatus.UpdatingComponentsSummary)
			changed = true
		}

		if len(prevStatus.UpdateStatus.BlockedComponentsSummary) != len(newStatus.UpdateStatus.BlockedComponentsSummary) {
			log.Info("UpdateStatus", "blockedComponentsSummary", newStatus.UpdateStatus.BlockedComponentsSummary)
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

func HaveClusterUpdateState(updateState ytv1.UpdateState) otypes.GomegaMatcher {
	return And(
		HaveClusterStateUpdating(),
		HaveField("Status.UpdateStatus.State", updateState),
	)
}

func HaveClusterUpdatingComponents(components ...consts.ComponentType) otypes.GomegaMatcher {
	return And(
		HaveClusterStateUpdating(),
		WithTransform(func(yts *ytv1.Ytsaurus) []consts.ComponentType {
			var result []consts.ComponentType
			for _, comp := range yts.Status.UpdateStatus.UpdatingComponents {
				result = append(result, comp.Type)
			}
			return result
		}, ConsistOf(components)),
	)
}

func HaveClusterUpdatingComponentsNames(components ...string) otypes.GomegaMatcher {
	return And(
		HaveClusterStateUpdating(),
		WithTransform(func(yts *ytv1.Ytsaurus) []string {
			var result []string
			for _, comp := range yts.Status.UpdateStatus.UpdatingComponents {
				result = append(result, comp.Name)
			}
			return result
		}, ConsistOf(components)),
	)
}

func HaveRemoteNodeReleaseStatusRunning() otypes.GomegaMatcher {
	return HaveField("Status.ReleaseStatus", ytv1.RemoteNodeReleaseStatusRunning)
}
