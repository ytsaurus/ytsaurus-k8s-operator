package components

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ytsaurusName = "testsaurus"
	domain       = "testdomain"

	scriptBefore = "SCRIPT"
	scriptAfter  = "UPDATED SCRIPT"
)

var (
	waitTimeout = 5 * time.Second
	waitTick    = 300 * time.Millisecond
)

func prepareTest(t *testing.T, namespace string) (*testutil.TestHelper, *apiproxy.Ytsaurus, *ytconfig.Generator) {
	h := testutil.NewTestHelper(t, namespace, filepath.Join("..", "..", "config", "crd", "bases"))
	h.Start(func(mgr ctrl.Manager) error { return nil })

	ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
	// Deploy of ytsaurus spec is required, so it could set valid owner references for child resources.
	testutil.DeployObject(h, &ytsaurusResource)

	scheme := runtime.NewScheme()
	utilruntime.Must(ytv1.AddToScheme(scheme))
	fakeRecorder := record.NewFakeRecorder(100)

	ytsaurus := apiproxy.NewYtsaurus(&ytsaurusResource, h.GetK8sClient(), fakeRecorder, scheme)
	cfgen := ytconfig.NewGenerator(ytsaurus.GetResource(), domain)
	return h, ytsaurus, cfgen
}

func syncJobUntilReady(t *testing.T, job *InitJob) {
	ctx := context.Background()

	require.Eventually(
		t,
		func() bool {
			err := job.Fetch(ctx)
			require.NoError(t, err)
			st, err := job.Sync(ctx, false)
			require.NoError(t, err)
			return st.SyncStatus == SyncStatusReady
		},
		waitTimeout,
		waitTick,
	)
}

func newTestJob(ytsaurus *apiproxy.Ytsaurus) *InitJob {
	k8sName := "dummy-name"
	return NewInitJob(
		&labeller.Labeller{
			ObjectMeta: &metav1.ObjectMeta{
				Name:      k8sName,
				Namespace: ytsaurus.GetResource().Namespace,
			},
			ComponentLabel: "ms",
			ComponentName:  k8sName,
		},
		ytsaurus.APIProxy(),
		ytsaurus,
		[]corev1.LocalObjectReference{},
		"dummy",
		consts.ClientConfigFileName,
		"dummy-image",
		func() ([]byte, error) { return []byte("dummy-cfg"), nil },
	)
}

func TestJobRestart(t *testing.T) {
	ctx := context.Background()

	namespace := "testjobrestart"
	h, ytsaurus, _ := prepareTest(t, namespace)
	// TODO: separate helper so no need to remember to call stop
	defer h.Stop()

	job := newTestJob(ytsaurus)
	job.SetInitScript(scriptBefore)
	syncJobUntilReady(t, job)

	err := job.prepareRestart(ctx, false)
	require.NoError(t, err)

	t.Log("Ensure job is deleted on restart")
	require.Eventually(t,
		func() bool {
			batchJob := batchv1.Job{}
			err = ytsaurus.APIProxy().Client().Get(ctx, client.ObjectKey{
				Name:      "ms-init-job-dummy",
				Namespace: namespace,
			}, &batchJob)
			return apierrors.IsNotFound(err)
		},
		waitTimeout,
		100*time.Millisecond,
	)

	t.Log("Wait for restart to be prepared.")
	require.Eventually(
		t,
		func() bool {
			job = newTestJob(ytsaurus)
			err = job.Fetch(ctx)
			require.NoError(t, err)
			return job.isRestartPrepared()
		},
		waitTimeout,
		waitTick,
	)
}

func TestJobScriptUpdateOnJobRestart(t *testing.T) {
	ctx := context.Background()

	namespace := "testjobscript"
	h, ytsaurus, _ := prepareTest(t, namespace)
	// TODO: separate helper so no need to remember to call stop
	defer h.Stop()

	job := newTestJob(ytsaurus)
	job.SetInitScript(scriptBefore)
	syncJobUntilReady(t, job)

	err := job.prepareRestart(ctx, false)
	require.NoError(t, err)

	require.Eventually(
		t,
		func() bool {
			job = newTestJob(ytsaurus)
			err = job.Fetch(ctx)
			require.NoError(t, err)
			return job.isRestartPrepared()
		},
		waitTimeout,
		waitTick,
	)

	// Imagine that new version of operator wants to set new init script for job.
	job = newTestJob(ytsaurus)
	job.SetInitScript(scriptAfter)
	syncJobUntilReady(t, job)

	cmData := testutil.FetchConfigMapData(
		h,
		"dummy-ms-init-job-config",
		consts.InitClusterScriptFileName,
	)
	require.Equal(t, scriptAfter, cmData)
}
