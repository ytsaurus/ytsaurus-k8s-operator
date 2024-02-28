package controllers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type testHelper struct {
	t          *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	k8sTestEnv *envtest.Environment
	k8sClient  client.Client
	cfg        *rest.Config
	namespace  string
}

func newTestHelper(t *testing.T, namespace string) *testHelper {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal(
			"KUBEBUILDER_ASSETS needed to be set for this test " +
				"Something like KUBEBUILDER_ASSETS=`bin/setup-envtest use 1.24.2 -p path` would do." +
				"Check Makefile for the details.",
		)
	}
	k8sTestEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
		ControlPlane: envtest.ControlPlane{},
	}

	testCtx, testCancel := context.WithCancel(context.Background())
	return &testHelper{
		t:          t,
		ctx:        testCtx,
		cancel:     testCancel,
		k8sTestEnv: k8sTestEnv,
		namespace:  namespace,
	}
}

func (h *testHelper) start() {
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
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	require.NoError(t, err)

	h.createNamespace()

	err = (&RemoteExecNodesReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("remoteexecnodes-controller"),
	}).SetupWithManager(mgr)
	require.NoError(t, err)

	go func() {
		err = mgr.Start(h.ctx)
		require.NoError(t, err)
	}()
}

func (h *testHelper) stop() {
	// Should cancel ctx before Stop
	// https://github.com/kubernetes-sigs/controller-runtime/issues/1571#issuecomment-945535598
	h.cancel()
	err := h.k8sTestEnv.Stop()
	require.NoError(h.t, err)
}

func (h *testHelper) getK8sClient() client.Client {
	if h.k8sClient == nil {
		k8sCli, err := client.New(h.cfg, client.Options{Scheme: scheme.Scheme})
		require.NoError(h.t, err)
		h.k8sClient = k8sCli
	}
	return h.k8sClient
}

func (h *testHelper) getObjectKey(name string) client.ObjectKey {
	return client.ObjectKey{Name: name, Namespace: h.namespace}
}

func (h *testHelper) getObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: h.namespace}
}

func (h *testHelper) createNamespace() {
	c := h.getK8sClient()
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: h.namespace,
		},
	}
	err := c.Create(context.Background(), &ns)
	require.NoError(h.t, err)
}
