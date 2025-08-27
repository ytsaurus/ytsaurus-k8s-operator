package components

import (
	"cmp"
	"context"
	"slices"
	"testing"

	cryptorand "crypto/rand"
	mathrand "math/rand"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	oformat "github.com/onsi/gomega/format"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"

	"k8s.io/client-go/tools/record"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

var log = logf.Log

func TestComponents(t *testing.T) {
	oformat.MaxLength = 20_000 // Do not truncate large YT errors
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test components")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})

func LogObjectEvents(ctx context.Context, k8sClient client.WithWatch, namespace string) func() {
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

func CompleteOneJob(ctx context.Context, k8sClient client.WithWatch, namespace string) *batchv1.Job {
	var jobs batchv1.JobList
	Expect(k8sClient.List(ctx, &jobs, &client.ListOptions{Namespace: namespace})).To(Succeed())
	for _, job := range jobs.Items {
		if job.Status.Succeeded != 0 {
			continue
		}
		log.Info("Complete job", "name", job.Name)
		job.Status.Succeeded = 1
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
		return &job
	}
	return nil
}

var _ = Describe("Components reconciler", Label("reconciler"), func() {
	var namespace string
	var ytBuilder *testutil.YtsaurusBuilder
	var ytsaurus *ytv1.Ytsaurus
	var ytsaurusKey client.ObjectKey
	var k8sScheme *runtime.Scheme
	var k8sEventBroadcaster record.EventBroadcaster
	var k8sEvents []corev1.Event
	var k8sClient client.WithWatch
	var statefulSets map[string]*appsv1.StatefulSet
	var configMaps map[string]*corev1.ConfigMap

	// NOTE: execution order for each test spec:
	// - BeforeEach               (configuration)
	// - JustBeforeEach           (creation, validation)
	// - It                       (test itself)
	// - JustAfterEach            (diagnosis, validation)
	// - AfterEach, DeferCleanup  (cleanup)
	//
	// See:
	// https://onsi.github.io/ginkgo/#separating-creation-and-configuration-justbeforeeach
	// https://onsi.github.io/ginkgo/#spec-cleanup-aftereach-and-defercleanup
	// https://onsi.github.io/ginkgo/#separating-diagnostics-collection-and-teardown-justaftereach

	BeforeEach(func(ctx context.Context) {
		By("Creating fake k8s client", func() {
			k8sScheme = runtime.NewScheme()
			Expect(clientgoscheme.AddToScheme(k8sScheme)).To(Succeed())
			Expect(ytv1.AddToScheme(k8sScheme)).To(Succeed())

			codecs := serializer.NewCodecFactory(k8sScheme)
			k8sObjectTraker := clienttesting.NewObjectTracker(k8sScheme, codecs.UniversalDeserializer())

			getGeneration := func(obj client.Object) int64 {
				gvr, _ := meta.UnsafeGuessKindToResource(obj.GetObjectKind().GroupVersionKind())
				currentObj, err := k8sObjectTraker.Get(gvr, obj.GetNamespace(), obj.GetName())
				Expect(err).To(Succeed())
				accessor, err := meta.Accessor(currentObj)
				Expect(err).To(Succeed())
				return accessor.GetGeneration()
			}

			clientBuilder := fake.NewClientBuilder()
			clientBuilder.WithScheme(k8sScheme)
			clientBuilder.WithObjectTracker(k8sObjectTraker)
			clientBuilder.WithStatusSubresource(&ytv1.Ytsaurus{})
			clientBuilder.WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					err := client.Get(ctx, key, obj, opts...)
					log.Info("Get object", "key", key, "ok", err == nil, "version", obj.GetResourceVersion())
					return err
				},
				Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					gvks, _, err := k8sScheme.ObjectKinds(obj)
					Expect(err).To(Succeed())
					obj.GetObjectKind().SetGroupVersionKind(gvks[0])
					obj.SetGeneration(1)

					log.Info("Create object", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName(), "version", obj.GetResourceVersion())
					return client.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					obj.SetGeneration(getGeneration(obj) + 1)

					log.Info("Update object", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName())
					return c.Update(ctx, obj, opts...)
				},
				Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					log.Info("Patch object", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName())
					return client.Patch(ctx, obj, patch, opts...)
				},
				Delete: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					log.Info("Delete object", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName())
					return client.Delete(ctx, obj, opts...)
				},
				SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					log.Info("SubResourceUpdate", "gvk", obj.GetObjectKind().GroupVersionKind(), "name", obj.GetName(), "subResourceName", subResourceName)
					return client.SubResource(subResourceName).Update(ctx, obj, opts...)
				},
			})
			k8sClient = clientBuilder.Build()

			k8sEventBroadcaster = record.NewBroadcasterForTests(0)
			eventWatcher := k8sEventBroadcaster.StartEventWatcher(func(event *corev1.Event) {
				log.Info("Event",
					"type", event.Type,
					"kind", event.InvolvedObject.Kind,
					"name", event.InvolvedObject.Name,
					"reason", event.Reason,
					"message", event.Message,
				)
				event.FirstTimestamp.Reset()
				event.LastTimestamp.Reset()
				event.Name = ""
				k8sEvents = append(k8sEvents, *event)
			})
			DeferCleanup(func() {
				eventWatcher.Stop()
			})
		})

		namespace = "ytsaurus-components"

		DeferCleanup(LogObjectEvents(ctx, k8sClient, namespace))

		By("Creating minimal Ytsaurus spec", func() {
			ytBuilder = &testutil.YtsaurusBuilder{
				MinReadyInstanceCount: ptr.To(0), // Do not wait any pods.
				Namespace:             namespace,
				YtsaurusImage:         testutil.YtsaurusImageCurrent,
				JobImage:              ptr.To(testutil.YtsaurusJobImage),
				QueryTrackerImage:     testutil.QueryTrackerImageCurrent,
			}
			ytBuilder.CreateMinimal()
			ytsaurus = ytBuilder.Ytsaurus
			ytsaurusKey = client.ObjectKey{Name: ytsaurus.Name, Namespace: namespace}
			ytsaurus.Status.State = ytv1.ClusterStateCreated
		})

		// generator = ytconfig.NewGenerator(ytsaurus, "cluster.local")
	})

	JustBeforeEach(func(ctx context.Context) {

		By("Creating namespace", func() {
			namespaceObject := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			Expect(k8sClient.Create(ctx, &namespaceObject)).Should(Succeed())
		})

		By("Creating ytsaurus resource", func() {
			if ytBuilder.Overrides != nil {
				Expect(k8sClient.Create(ctx, ytBuilder.Overrides)).To(Succeed())
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).To(Succeed())
		})

		By("Running ytsaurus reconciler", func() {
			ytsaurusReconciler := controllers.YtsaurusReconciler{
				ClusterDomain: "cluster.local",
				Client:        k8sClient,
				Scheme:        k8sScheme,
				Recorder:      k8sEventBroadcaster.NewRecorder(k8sScheme, corev1.EventSource{Component: "ytsaurus"}),
			}

			// Override crypto/rand reader to generate deterministic tokens.
			{
				origReader := cryptorand.Reader
				defer func() {
					cryptorand.Reader = origReader
				}()
				cryptorand.Reader = mathrand.New(mathrand.NewSource(42))
			}

			k8sEvents = nil

			Eventually(ctx, func(ctx context.Context) (bool, error) {
				result, err := ytsaurusReconciler.Sync(ctx, ytsaurus)

				CompleteOneJob(ctx, k8sClient, namespace)

				return result.Requeue || result.RequeueAfter > 0, err
			}, "60s", "0ms").Should(BeFalse())
		})

		By("Getting ytsaurus resource", func() {
			Expect(k8sClient.Get(ctx, ytsaurusKey, ytsaurus)).To(Succeed())
			Expect(ytsaurus.Status.State).To(Equal(ytv1.ClusterStateRunning))
		})

	})

	JustBeforeEach(func(ctx context.Context) {
		By("Checking Ytsaurus spec", func() {
			for i := range ytsaurus.Status.Conditions {
				ytsaurus.Status.Conditions[i].LastTransitionTime.Reset()
			}
			canonize.AssertStruct(GinkgoT(), "Ytsaurus", ytsaurus)
		})

		var objectList []metav1.ObjectMeta

		By("Checking Events", func() {
			canonize.AssertStruct(GinkgoT(), "Events", k8sEvents)
		})

		By("Checking Services", func() {
			var objList corev1.ServiceList
			Expect(k8sClient.List(ctx, &objList)).To(Succeed())
			for i := range objList.Items {
				obj := &objList.Items[i]
				log.Info("Found Service", "name", obj.Name)
				objectList = append(objectList, obj.ObjectMeta)

				canonize.AssertStruct(GinkgoT(), "Service "+obj.Name, obj)
			}
		})

		By("Checking StatefulSets", func() {
			statefulSets = make(map[string]*appsv1.StatefulSet)
			var objList appsv1.StatefulSetList
			Expect(k8sClient.List(ctx, &objList)).To(Succeed())
			for i := range objList.Items {
				obj := &objList.Items[i]
				log.Info("Found StatefulSet", "name", obj.Name)
				objectList = append(objectList, obj.ObjectMeta)
				statefulSets[obj.Name] = obj

				canonize.AssertStruct(GinkgoT(), "StatefulSet "+obj.Name, obj)
			}
		})

		By("Checking Deployments", func() {
			var objList appsv1.DeploymentList
			Expect(k8sClient.List(ctx, &objList)).To(Succeed())
			for i := range objList.Items {
				obj := &objList.Items[i]
				log.Info("Found Deployment", "name", obj.Name)
				objectList = append(objectList, obj.ObjectMeta)

				canonize.AssertStruct(GinkgoT(), "Deployment "+obj.Name, obj)
			}
		})

		By("Checking ConfigMaps", func() {
			configMaps = make(map[string]*corev1.ConfigMap)
			var objList corev1.ConfigMapList
			Expect(k8sClient.List(ctx, &objList)).To(Succeed())
			for i := range objList.Items {
				obj := &objList.Items[i]
				log.Info("Found ConfigMap", "name", obj.Name)
				objectList = append(objectList, obj.ObjectMeta)
				configMaps[obj.Name] = obj

				canonize.AssertStruct(GinkgoT(), "ConfigMap "+obj.Name, obj)
			}
		})

		By("Checking Jobs", func() {
			var objList batchv1.JobList
			Expect(k8sClient.List(ctx, &objList)).To(Succeed())
			for i := range objList.Items {
				obj := &objList.Items[i]
				log.Info("Found Job", "name", obj.Name)
				objectList = append(objectList, obj.ObjectMeta)

				canonize.AssertStruct(GinkgoT(), "Job "+obj.Name, obj)
			}
		})

		By("Checking object list", func() {
			slices.SortStableFunc(objectList, func(a, b metav1.ObjectMeta) int {
				return cmp.Compare(a.Name, b.Name)
			})
			canonize.AssertStruct(GinkgoT(), "Object List", objectList)
		})
	})

	Context("Minimal", func() {
		It("Test", func(ctx context.Context) {})
	})

	Context("With all components", func() {
		BeforeEach(func() {
			ytBuilder.WithMasterCaches()
			ytBuilder.WithRPCProxies()
			ytBuilder.WithDataNodes()
			// FIXME(khlebnikov): Do not bootstrap tablet cell bundles when not asked.
			// ytBuilder.WithTabletNodes()
			ytBuilder.WithScheduler()
			ytBuilder.WithControllerAgents()
			ytBuilder.WithExecNodes()
			ytBuilder.WithQueryTracker()
			ytBuilder.WithQueueAgent()
			ytBuilder.WithYqlAgent()
		})
		It("Test", func(ctx context.Context) {})
	})

	Context("With CRI job environment", func() {
		BeforeEach(func() {
			ytBuilder.WithExecNodes()
			ytBuilder.WithCRIJobEnvironment()
			ytBuilder.WithOverrides()
		})
		It("Test", func(ctx context.Context) {})
	})

	Context("With CRI job environment - CRI-O", Label("crio"), func() {
		BeforeEach(func() {
			ytBuilder.CRIService = ptr.To(ytv1.CRIServiceCRIO)
			ytBuilder.WithExecNodes()
			ytBuilder.WithCRIJobEnvironment()
		})
		It("Test", func(ctx context.Context) {})
	})
})
