package components

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
)

const (
	scriptBefore = "SCRIPT"
	scriptAfter  = "UPDATED SCRIPT"
)

var (
	waitTimeout = 5 * time.Second
	waitTick    = 300 * time.Millisecond
)

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
