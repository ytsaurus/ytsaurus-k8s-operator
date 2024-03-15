package controllers_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/controllers"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
)

const (
	ytsaurusName      = "testsaurus"
	testYtsaurusImage = "test-ytsaurus-image"
	dndsNameOne       = "dn-1"
)

func TestYtsaurusFromScratch(t *testing.T) {
	namespace := "ytsaurus-from-scratch"
	h := testutil.NewTestHelper(t, namespace)
	reconcilerSetup := func(mgr ctrl.Manager) error {
		return (&controllers.YtsaurusReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ytsaurus-controller"),
		}).SetupWithManager(mgr)
	}
	h.Start(reconcilerSetup)
	defer h.Stop()

	ytsaurusResource := buildMinimalYtsaurus(namespace, ytsaurusName)
	testutil.DeployObject(h, &ytsaurusResource)

	// emulate master init job succeeded
	testutil.MarkJobSucceeded(h, "yt-master-init-job-default")
	testutil.MarkJobSucceeded(h, "yt-client-init-job-user")

	for _, compName := range []string{
		"discovery",
		"master",
		"http-proxy",
	} {
		testutil.FetchAndCheckConfigMapContainsEventually(
			h,
			"yt-"+compName+"-config",
			"ytserver-"+compName+".yson",
			"ms-0.masters."+namespace+".svc.cluster.local:9010",
		)
	}
	testutil.FetchAndCheckConfigMapContainsEventually(
		h,
		"yt-data-node-"+dndsNameOne+"-config",
		"ytserver-data-node.yson",
		"ms-0.masters."+namespace+".svc.cluster.local:9010",
	)

	for _, stsName := range []string{
		"ds",
		"ms",
		"hp",
		"dnd-" + dndsNameOne,
	} {
		testutil.FetchEventually(
			h,
			stsName,
			&appsv1.StatefulSet{},
		)
	}

	testutil.FetchAndCheckEventually(
		h,
		"yt-client-secret",
		&corev1.Secret{},
		"secret with not empty token",
		func(obj client.Object) bool {
			secret := obj.(*corev1.Secret)
			return len(secret.Data["YT_TOKEN"]) != 0
		},
	)

	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"cluster state is running",
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}

func TestYtsaurusUpdateMasterImage(t *testing.T) {
	namespace := "upd-master-image"
	h := testutil.NewTestHelper(t, namespace)
	reconcilerSetup := func(mgr ctrl.Manager) error {
		return (&controllers.YtsaurusReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ytsaurus-controller"),
		}).SetupWithManager(mgr)
	}
	h.Start(reconcilerSetup)
	defer h.Stop()

	ytsaurusInMemory := testutil.NewYtsaurusInMemory(t.Log)
	require.NoError(t, os.Setenv("YTOP_PROXY", "127.0.0.1:9999"))
	go func() {
		err := ytsaurusInMemory.Start(9999)
		require.NoError(t, err)
	}()

	//h.ytsaurusInMemory.Set("//sys/@hydra_read_only", false)
	ytsaurusResource := buildMinimalYtsaurus(namespace, ytsaurusName)
	testutil.DeployObject(h, &ytsaurusResource)

	// emulate master init job succeeded
	testutil.MarkJobSucceeded(h, "yt-master-init-job-default")

	// emulate tablet cells recovered
	//h.ytsaurusInMemory.Set("//sys/tablet_cell_bundles", map[string]any{"sys": nil})
	ytsaurusInMemory.Set("//sys/tablet_cells", map[string]any{
		"1-602-2bc-955ed415": nil,
	})
	//h.ytsaurusInMemory.Set("//sys/tablet_cells/1-602-2bc-955ed415/@tablet_cell_bundle", "sys")

	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"cluster state is running",
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)

	imageUpdated := testYtsaurusImage + "-updated"
	ytsaurusResource.Spec.PrimaryMasters.Image = &imageUpdated
	t.Log("[ Updating master ]")
	testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	// expect would block on full update
	// TODO: support blocked state?
	//testutil.FetchAndCheckEventually(
	//	h,
	//	ytsaurusName,
	//	&ytv1.Ytsaurus{},
	//	"cluster state is update blocked",
	//	func(obj client.Object) bool {
	//		state := obj.(*ytv1.Ytsaurus).Status.State
	//		return state == ytv1.ClusterStateCancelUpdate
	//	},
	//)

	// Making full update possible.
	//h.ytsaurusInMemory.Set("//sys/tablet_cell_bundles/sys/@health", "good")
	// HandlePossibilityCheck: lost vital chunks check
	ytsaurusInMemory.Set("//sys/lost_vital_chunks/@count", 0)
	// HandlePossibilityCheck: quorum missing chunks check
	ytsaurusInMemory.Set("//sys/quorum_missing_chunks/@count", 0)
	// HandlePossibilityCheck: master activity check
	const (
		ms0 = "ms-0:9010"
		ms1 = "ms-1:9010"
		ms2 = "ms-2:9010"
	)
	masterAddressesList := []string{ms0, ms1, ms2}
	ytsaurusInMemory.Set(
		"//sys/primary_masters/"+ms0+"/orchid/monitoring/hydra",
		map[string]any{"active": true, "state": "leading"},
	)
	ytsaurusInMemory.Set(
		"//sys/primary_masters/"+ms1+"/orchid/monitoring/hydra",
		map[string]any{"active": true, "state": "following"},
	)
	ytsaurusInMemory.Set(
		"//sys/primary_masters/"+ms2+"/orchid/monitoring/hydra",
		map[string]any{"active": true, "state": "following"},
	)
	masterAddresses := map[string]any{
		ms0: nil,
		ms1: nil,
		ms2: nil,
	}
	ytsaurusInMemory.Set("//sys/primary_masters", masterAddresses)

	// SaveMasterMonitoringPaths
	ytsaurusInMemory.Set("//sys/@cluster_connection/primary_master", map[string]any{
		"cell_id":   "1",
		"addresses": masterAddressesList,
	})
	var masterMonitoringPaths []string
	for _, ms := range masterAddressesList {
		path := "//sys/cluster_masters/" + ms + "/orchid/monitoring/hydra"
		// emulate not read only before update
		ytsaurusInMemory.Set(path, map[string]any{
			"read_only": false,
		})
		masterMonitoringPaths = append(masterMonitoringPaths, path)
	}
	t.Log("[ Enable Full Update ]")
	ytsaurusResource.Spec.EnableFullUpdate = true
	testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"expected master monitoring paths count",
		func(obj client.Object) bool {
			status := obj.(*ytv1.Ytsaurus).Status
			if len(status.UpdateStatus.MasterMonitoringPaths) == len(masterMonitoringPaths) {
				t.Log("finish waiting master monitoring paths count")
				return true
			}
			return false
		},
	)

	t.Log("[ Emulate startBuildMasterSnapshots executed ]")
	for _, path := range masterMonitoringPaths {
		ytsaurusInMemory.Set(path, map[string]any{
			"read_only":               true,
			"last_snapshot_read_only": true,
		})
	}

	//t.Log("[ Emulate master exit read only succeeded ]")
	//testutil.FetchAndCheckEventually(
	//	h,
	//	ytsaurusName,
	//	&ytv1.Ytsaurus{},
	//	"master snapshot is done",
	//	func(obj client.Object) bool {
	//		status := obj.(*ytv1.Ytsaurus).Status
	//		return meta.IsStatusConditionTrue(status.UpdateStatus.Conditions, string(ytflow.MasterExitReadOnlyStep)+"Run")
	//	},
	//)
	//testutil.MarkJobSucceeded(h, "yt-master-init-job-exit-read-only")

	t.Log("[ Wait for YTsaurus Running status ]")
	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"cluster state is running",
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}

func buildMinimalYtsaurus(namespace, name string) ytv1.Ytsaurus {
	return ytv1.Ytsaurus{
		ObjectMeta: v1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: ytv1.YtsaurusSpec{
			CommonSpec: ytv1.CommonSpec{
				CoreImage:     testYtsaurusImage,
				UseShortNames: true,
			},
			IsManaged:        true,
			EnableFullUpdate: false,

			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 3,
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 3,
					Locations: []ytv1.LocationSpec{
						{
							LocationType: "MasterChangelogs",
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: "MasterSnapshots",
							Path:         "/yt/master-data/master-snapshots",
						},
					},
				},
				MasterConnectionSpec: ytv1.MasterConnectionSpec{
					CellTag: 1,
				},
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3},
					ServiceType:  corev1.ServiceTypeNodePort,
				},
			},
			DataNodes: []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 5,
						Locations: []ytv1.LocationSpec{
							{
								LocationType: "ChunkStore",
								Path:         "/yt/node-data/chunk-store",
							},
						},
					},
					ClusterNodesSpec: ytv1.ClusterNodesSpec{},
					Name:             dndsNameOne,
				},
			},
		},
	}
}
