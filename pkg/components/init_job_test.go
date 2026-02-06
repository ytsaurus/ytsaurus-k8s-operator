package components

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
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
	h := testutil.NewTestHelper(t, namespace, "../..")
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
	resource := ytsaurus.GetResource()
	return NewInitJob(
		&labeller.Labeller{
			Namespace:     resource.GetNamespace(),
			ResourceName:  resource.GetName(),
			ClusterName:   resource.GetName(),
			ComponentType: consts.MasterType,
		},
		ytsaurus,
		ytsaurus,
		"dummy",
		consts.ClientConfigFileName,
		func() ([]byte, error) { return []byte("dummy-cfg"), nil },
		&resource.Spec.CommonSpec,
		&resource.Spec.PodSpec,
		&ytv1.InstanceSpec{
			Image: ptr.To("dummy-image"),
		},
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
			err = ytsaurus.Client().Get(ctx, client.ObjectKey{
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
		"dummy-yt-master-init-job-config",
		consts.InitClusterScriptFileName,
	)
	require.Equal(t, scriptAfter, cmData)
}
