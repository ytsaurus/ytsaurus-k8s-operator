package components

import (
	"context"
	"os"
	"time"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	batchv1 "k8s.io/api/batch/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

const (
	ytsaurusName = "testsaurus"

	scriptBefore = "SCRIPT"
	scriptAfter  = "UPDATED SCRIPT"

	waitTimeout = 5 * time.Second
	waitTick    = 300 * time.Millisecond
)

var logger = GinkgoLogr

func syncJobUntilReady(job *InitJob) {
	ctx := context.Background()

	Eventually(func() SyncStatus {
		err := job.Fetch(ctx)
		Expect(err).NotTo(HaveOccurred())
		st, err := job.Sync(ctx, false)
		logger.Info("Job status", "state", st.SyncStatus, "message", st.Message)
		Expect(err).NotTo(HaveOccurred())
		return st.SyncStatus
	}, waitTimeout, waitTick).Should(Equal(SyncStatusReady))
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

var _ = Describe("InitJob", func() {
	var h *testutil.TestHelper
	var ytsaurus *apiproxy.Ytsaurus
	var namespace string

	JustBeforeEach(func() {
		if os.Getenv("CANONIZE") != "" {
			Skip("Nothing to CANONIZE")
		}

		h = testutil.NewTestHelper(GinkgoTB(), namespace, "../..")
		h.Start(func(mgr ctrl.Manager) error { return nil })

		ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
		// Deploy of ytsaurus spec is required, so it could set valid owner references for child resources.
		testutil.DeployObject(h, &ytsaurusResource)

		scheme := runtime.NewScheme()
		utilruntime.Must(ytv1.AddToScheme(scheme))
		fakeRecorder := record.NewFakeRecorder(100)

		ytsaurus = apiproxy.NewYtsaurus(&ytsaurusResource, h.GetK8sClient(), fakeRecorder, scheme)
	})

	Describe("Job restart", func() {
		BeforeEach(func() {
			namespace = "testjobrestart"
		})

		It("should delete job and prepare restart", func(ctx context.Context) {
			job := newTestJob(ytsaurus)
			job.SetInitScript(scriptBefore)
			syncJobUntilReady(job)

			job.Restart()

			By("Ensure job is deleted on restart")
			Eventually(func() bool {
				batchJob := batchv1.Job{}
				err := ytsaurus.Client().Get(ctx, client.ObjectKey{
					Name:      "ms-init-job-dummy",
					Namespace: namespace,
				}, &batchJob)
				return apierrors.IsNotFound(err)
			}, waitTimeout, 100*time.Millisecond).Should(BeTrueBecause("init job should be deleted after restart"))
		})
	})

	Describe("Job script update", func() {
		BeforeEach(func() {
			namespace = "testjobscript"
		})

		It("should update script on job restart", func(ctx context.Context) {
			job := newTestJob(ytsaurus)
			job.SetInitScript(scriptBefore)
			syncJobUntilReady(job)


			// Imagine that new version of operator wants to set new init script for job.
			job = newTestJob(ytsaurus)
			job.SetInitScript(scriptAfter)
			job.Restart()
			syncJobUntilReady(job)

			cmData := testutil.FetchConfigMapData(
				h,
				"yt-master-init-job-dummy-config",
				consts.InitJobScriptFileName,
			)
			Expect(cmData).To(Equal(scriptAfter))
		})
	})
})
