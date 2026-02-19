package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr/testr"

	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

const (
	eventuallyWaitTime = 20 * time.Second
	eventuallyTickTime = 200 * time.Millisecond
)

// TODO: Remove all this and use helpers written for e2e tests.

type TestHelper struct {
	t          testing.TB
	k8sTestEnv *envtest.Environment
	scheme     *runtime.Scheme
	client     client.Client
	cfg        *rest.Config
	ticker     *time.Ticker
	Namespace  string
}

func GetenvOr(key, value string) string {
	if x, ok := os.LookupEnv(key); ok {
		return x
	}
	return value
}

func NewTestHelper(t testing.TB, namespace, topDirectoryPath string) *TestHelper {
	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: filepath.Join(topDirectoryPath, "bin", "envtest-assets"),
		CRDDirectoryPaths:     []string{filepath.Join(topDirectoryPath, "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
		ControlPlane:             envtest.ControlPlane{},
		AttachControlPlaneOutput: true,
		ControlPlaneStartTimeout: 60 * time.Second,
		ControlPlaneStopTimeout:  60 * time.Second,
	}

	apiServer := testEnv.ControlPlane.GetAPIServer()
	apiServer.Configure().Set("advertise-address", "127.0.0.1")

	return &TestHelper{
		t:          t,
		k8sTestEnv: testEnv,
		Namespace:  namespace,
		ticker:     time.NewTicker(200 * time.Millisecond),
	}
}

func (h *TestHelper) Start(reconcilerSetup func(mgr ctrl.Manager) error) {
	t := h.t

	logger := testr.NewWithInterface(t, testr.Options{})
	logf.SetLogger(logger)

	conf, err := h.k8sTestEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() {
		Expect(h.k8sTestEnv.Stop()).NotTo(HaveOccurred())
	})

	Expect(conf).NotTo(BeNil())
	h.cfg = conf

	h.scheme = runtime.NewScheme()

	err = clientgoscheme.AddToScheme(h.scheme)
	Expect(err).NotTo(HaveOccurred())

	err = ytv1.AddToScheme(h.scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sCli, err := client.New(h.cfg, client.Options{Scheme: h.scheme})
	Expect(err).NotTo(HaveOccurred())
	h.client = k8sCli

	mgr, err := ctrl.NewManager(conf, ctrl.Options{
		Logger: logger,
		Scheme: h.scheme,
		// To get rid of macOS' accept incoming network connections popup
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
		Controller: config.Controller{
			// Tests register controllers more than once.
			SkipNameValidation: ptr.To(true),
			// Tests must crash instantly.
			RecoverPanic: ptr.To(false),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	h.createNamespace()

	err = reconcilerSetup(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		for range h.ticker.C {
			MarkAllJobsCompleted(h)
		}
	}()
	t.Cleanup(h.ticker.Stop)

	// Stop controller before stopping environment.
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1571#issuecomment-945535598
	mgrCtx, mgrStop := context.WithCancel(context.Background())
	t.Cleanup(mgrStop)

	go func() {
		Expect(mgr.Start(mgrCtx)).NotTo(HaveOccurred())
	}()
}

func (h *TestHelper) GetK8sClient() client.Client {
	return h.client
}

func (h *TestHelper) GetObjectKey(name string) client.ObjectKey {
	return client.ObjectKey{Name: name, Namespace: h.Namespace}
}

func (h *TestHelper) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: h.Namespace}
}

func (h *TestHelper) createNamespace() {
	c := h.GetK8sClient()
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.Namespace,
		},
	}
	err := c.Create(context.Background(), &ns)
	Expect(err).NotTo(HaveOccurred())
}

func (h *TestHelper) Log(args ...any) {
	h.t.Log(args...)
}

func (h *TestHelper) Logf(format string, args ...any) {
	h.t.Logf(format, args...)
}

// helpers
func GetObject(h *TestHelper, key string, emptyObject client.Object) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.Get(context.Background(), h.GetObjectKey(key), emptyObject)
	Expect(err).NotTo(HaveOccurred())
}
func ListObjects(h *TestHelper, emptyList client.ObjectList) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.List(context.Background(), emptyList)
	Expect(err).NotTo(HaveOccurred())
}
func DeployObject(h *TestHelper, object client.Object) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.Create(context.Background(), object)
	Expect(err).NotTo(HaveOccurred())
}
func UpdateObject(h *TestHelper, emptyObject, newObject client.Object) {
	k8sCli := h.GetK8sClient()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		GetObject(h, newObject.GetName(), emptyObject)
		newObject.SetResourceVersion(emptyObject.GetResourceVersion())
		return k8sCli.Update(context.Background(), newObject)
	})
	Expect(err).NotTo(HaveOccurred())
}
func UpdateObjectStatus(h *TestHelper, newObject client.Object) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.Status().Update(context.Background(), newObject)
	Expect(err).NotTo(HaveOccurred())
}

func MarkJobSucceeded(h *TestHelper, key string) {
	job := &batchv1.Job{}
	FetchEventually(h, key, job)
	job.Status.Succeeded = 1
	UpdateObjectStatus(h, job)
}

func MarkAllJobsCompleted(h *TestHelper) {
	jobs := &batchv1.JobList{}
	err := h.GetK8sClient().List(context.Background(), jobs)
	Expect(err).NotTo(HaveOccurred())
	for i := range jobs.Items {
		job := &jobs.Items[i]
		if job.Status.Succeeded == 0 {
			h.t.Logf("found job %s, marking as completed", job.Name)
			job.Status.Succeeded = 1
			UpdateObjectStatus(h, job)
		}
		if !job.DeletionTimestamp.IsZero() {
			h.t.Logf(
				"found job %s, with deletion ts %s and finalizers: %s. Deleting as gc would do in real cluster.",
				job.Name,
				job.DeletionTimestamp,
				job.Finalizers,
			)
			job.Finalizers = []string{}
			UpdateObject(h, &batchv1.Job{}, job)
			err = h.client.Delete(context.Background(), job)
			if err != nil && !apierrors.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}
}

func FetchEventually(h *TestHelper, key string, obj client.Object) {
	FetchAndCheckEventually(
		h,
		key,
		obj,
		"object successfully fetched",
		func(obj client.Object) bool {
			return true
		})
}

func FetchAndCheckEventually(
	h *TestHelper,
	key string,
	obj client.Object,
	condDescription string,
	condition func(obj client.Object) bool,
) {
	h.t.Log("Waiting condition", condDescription, "for resource", key)
	Eventually(func() (client.Object, error) {
		err := h.GetK8sClient().Get(h.t.Context(), h.GetObjectKey(key), obj)
		return obj, err
	}, eventuallyWaitTime, eventuallyTickTime).Should(Satisfy(condition), condDescription)
}

func FetchAndCheckConfigMapContainsEventually(h *TestHelper, objectKey, cmKey, expectSubstr string) {
	var cmData map[string]string
	FetchAndCheckEventually(
		h,
		objectKey,
		&corev1.ConfigMap{},
		"config map exists",
		func(obj client.Object) bool {
			cmData = obj.(*corev1.ConfigMap).Data
			return true
		},
	)
	Expect(cmData).To(HaveKey(cmKey))
	ysonContent := cmData[cmKey]
	Expect(ysonContent).To(ContainSubstring(expectSubstr))
}

func FetchConfigMapData(h *TestHelper, objectKey, mapKey string) string {
	configMap := corev1.ConfigMap{}
	FetchEventually(h, objectKey, &configMap)
	data := configMap.Data
	ysonNodeConfig := data[mapKey]
	return ysonNodeConfig
}
