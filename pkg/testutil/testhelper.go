package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type TestHelper struct {
	t          *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	k8sTestEnv *envtest.Environment
	k8sClient  client.Client
	cfg        *rest.Config
	ticker     *time.Ticker
	Namespace  string
}

func NewTestHelper(t *testing.T, namespace, crdDirectoryPath string) *TestHelper {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal(
			"KUBEBUILDER_ASSETS needed to be set for this test " +
				"Something like KUBEBUILDER_ASSETS=`bin/setup-envtest use 1.24.2 -p path` would do." +
				"Check Makefile for the details.",
		)
	}
	k8sTestEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{crdDirectoryPath},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
		ControlPlane: envtest.ControlPlane{},
	}

	testCtx, testCancel := context.WithCancel(context.Background())

	return &TestHelper{
		t:          t,
		ctx:        testCtx,
		cancel:     testCancel,
		k8sTestEnv: k8sTestEnv,
		Namespace:  namespace,
		ticker:     time.NewTicker(200 * time.Millisecond),
	}
}

func (h *TestHelper) Start(reconcilerSetup func(mgr ctrl.Manager) error) {
	t := h.t
	conf, err := h.k8sTestEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, conf)
	h.cfg = conf

	err = ytv1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	mgr, err := ctrl.NewManager(conf, ctrl.Options{
		Scheme: scheme.Scheme,
		// To get rid of macOS' accept incoming network connections popup
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
		Logger:                 testr.New(t),
	})
	require.NoError(t, err)

	h.createNamespace()

	err = reconcilerSetup(mgr)
	require.NoError(t, err)

	go func() {
		err = mgr.Start(h.ctx)
		require.NoError(t, err)
	}()

	go func() {
		for range h.ticker.C {
			MarkAllJobsCompleted(h)
		}
	}()
}

func (h *TestHelper) Stop() {
	h.ticker.Stop()
	// Should cancel ctx before Stop
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1571#issuecomment-945535598
	h.cancel()
	err := h.k8sTestEnv.Stop()
	require.NoError(h.t, err)
}

func (h *TestHelper) GetK8sClient() client.Client {
	if h.k8sClient == nil {
		k8sCli, err := client.New(h.cfg, client.Options{Scheme: scheme.Scheme})
		require.NoError(h.t, err)
		h.k8sClient = k8sCli
	}
	return h.k8sClient
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
	require.NoError(h.t, err)
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
	require.NoError(h.t, err)
}
func ListObjects(h *TestHelper, emptyList client.ObjectList) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.List(context.Background(), emptyList)
	require.NoError(h.t, err)
}
func DeployObject(h *TestHelper, object client.Object) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.Create(context.Background(), object)
	require.NoError(h.t, err)
}
func UpdateObject(h *TestHelper, emptyObject, newObject client.Object) {
	k8sCli := h.GetK8sClient()

	GetObject(h, newObject.GetName(), emptyObject)

	newObject.SetResourceVersion(emptyObject.GetResourceVersion())
	err := k8sCli.Update(context.Background(), newObject)
	require.NoError(h.t, err)
}
func UpdateObjectStatus(h *TestHelper, newObject client.Object) {
	k8sCli := h.GetK8sClient()
	err := k8sCli.Status().Update(context.Background(), newObject)
	require.NoError(h.t, err)
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
	require.NoError(h.t, err)
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
			err = h.k8sClient.Delete(context.Background(), job)
			if err != nil && !apierrors.IsNotFound(err) {
				panic(fmt.Sprintf("failed to delete job %s", job))
			}
		}
	}
}

const (
	eventuallyWaitTime = 10 * time.Second
	eventuallyTickTime = 500 * time.Millisecond
)

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
func FetchAndCheckEventually(h *TestHelper, key string, obj client.Object, condDescription string, condition func(obj client.Object) bool) {
	k8sCli := h.GetK8sClient()
	Eventually(
		h,
		fmt.Sprintf("condition '%s' for resource: `%s`", condDescription, key),
		func() bool {
			err := k8sCli.Get(context.Background(), h.GetObjectKey(key), obj)
			if err != nil {
				if apierrors.IsNotFound(err) {
					h.t.Logf("object %v not found.", key)
					return false
				}
				require.NoError(h.t, err)
			}
			return condition(obj)
		},
	)
}
func Eventually(h *TestHelper, description string, condition func() bool) {
	h.t.Log("start waiting " + description)
	waitTime := eventuallyWaitTime
	// Useful when debugging test.
	waitTimeFromEnv := os.Getenv("TEST_EVENTUALLY_WAIT_TIME")
	if waitTimeFromEnv != "" {
		var err error
		waitTime, err = time.ParseDuration(waitTimeFromEnv)
		if err != nil {
			h.t.Fatalf("failed to parse TEST_EVENTUALLY_WAIT_TIME=%s", waitTimeFromEnv)
		}
	}
	require.Eventually(
		h.t,
		condition,
		waitTime,
		eventuallyTickTime,
	)
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
	require.Contains(h.t, cmData, cmKey)
	ysonContent := cmData[cmKey]
	require.Contains(h.t, ysonContent, expectSubstr)
}

func FetchConfigMapData(h *TestHelper, objectKey, mapKey string) string {
	configMap := corev1.ConfigMap{}
	FetchEventually(h, objectKey, &configMap)
	data := configMap.Data
	ysonNodeConfig := data[mapKey]
	return ysonNodeConfig
}
