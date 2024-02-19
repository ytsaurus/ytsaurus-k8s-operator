package controllers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

const (
	ytsaurusName      = "testsaurus"
	testYtsaurusImage = "test-ytsaurus-image"
	dndsNameOne       = "dn-1"
)

func TestYtsaurusFromScratch(t *testing.T) {
	require.NoError(t, os.Setenv("ENABLE_NEW_FLOW", "true"))
	namespace := "ytsaurus-from-scratch"
	h := newTestHelper(t, namespace)
	h.start()
	defer h.stop()

	h.ytsaurusInMemory.Set("//sys/@hydra_read_only", false)

	ytsaurusResource := buildMinimalYtsaurus(namespace, ytsaurusName)
	deployObject(h, &ytsaurusResource)

	// emulate master init job succeeded
	markJobSucceeded(h, "yt-master-init-job-default")

	for _, compName := range []string{
		"discovery",
		"master",
		"http-proxy",
	} {
		fetchAndCheckConfigMapContainsEventually(
			h,
			"yt-"+compName+"-config",
			"ytserver-"+compName+".yson",
			"ms-0.masters."+namespace+".svc.cluster.local:9010",
		)
	}
	fetchAndCheckConfigMapContainsEventually(
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
		fetchEventually(
			h,
			stsName,
			&appsv1.StatefulSet{},
		)
	}

	fetchAndCheckEventually(
		h,
		"yt-client-secret",
		&corev1.Secret{},
		func(obj client.Object) bool {
			secret := obj.(*corev1.Secret)
			return len(secret.Data["YT_TOKEN"]) != 0
		},
	)

	//// emulate tablet cells recovered
	//h.ytsaurusInMemory.Set("//sys/tablet_cells", map[string]any{
	//	"1-602-2bc-955ed415": nil,
	//})
	//h.ytsaurusInMemory.Set("//sys/tablet_cells/1-602-2bc-955ed415/@tablet_cell_bundle", "sys")

	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}

func TestYtsaurusUpdateMasterImage(t *testing.T) {
	require.NoError(t, os.Setenv("ENABLE_NEW_FLOW", "true"))
	namespace := "upd-master-image"
	h := newTestHelper(t, namespace)
	h.start()
	defer h.stop()

	//h.ytsaurusInMemory.Set("//sys/@hydra_read_only", false)
	ytsaurusResource := buildMinimalYtsaurus(namespace, ytsaurusName)
	deployObject(h, &ytsaurusResource)

	// emulate master init job succeeded
	markJobSucceeded(h, "yt-master-init-job-default")

	// emulate tablet cells recovered
	//h.ytsaurusInMemory.Set("//sys/tablet_cell_bundles", map[string]any{"sys": nil})
	h.ytsaurusInMemory.Set("//sys/tablet_cells", map[string]any{
		"1-602-2bc-955ed415": nil,
	})
	//h.ytsaurusInMemory.Set("//sys/tablet_cells/1-602-2bc-955ed415/@tablet_cell_bundle", "sys")

	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)

	imageUpdated := testYtsaurusImage + "-updated"
	ytsaurusResource.Spec.PrimaryMasters.Image = &imageUpdated
	t.Log("[ Updating master ]")
	updateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	// expect would block on full update
	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateCancelUpdate
		},
	)

	// Making full update possible.
	//h.ytsaurusInMemory.Set("//sys/tablet_cell_bundles/sys/@health", "good")
	// HandlePossibilityCheck: lost vital chunks check
	h.ytsaurusInMemory.Set("//sys/lost_vital_chunks/@count", 0)
	// HandlePossibilityCheck: quorum missing chunks check
	h.ytsaurusInMemory.Set("//sys/quorum_missing_chunks/@count", 0)
	// HandlePossibilityCheck: master activity check
	const (
		ms0 = "ms-0:9010"
		ms1 = "ms-1:9010"
		ms2 = "ms-2:9010"
	)
	masterAddressesList := []string{ms0, ms1, ms2}
	h.ytsaurusInMemory.Set(
		"//sys/primary_masters/"+ms0+"/orchid/monitoring/hydra",
		map[string]any{"active": true, "state": "leading"},
	)
	h.ytsaurusInMemory.Set(
		"//sys/primary_masters/"+ms1+"/orchid/monitoring/hydra",
		map[string]any{"active": true, "state": "following"},
	)
	h.ytsaurusInMemory.Set(
		"//sys/primary_masters/"+ms2+"/orchid/monitoring/hydra",
		map[string]any{"active": true, "state": "following"},
	)
	masterAddresses := map[string]any{
		ms0: nil,
		ms1: nil,
		ms2: nil,
	}
	h.ytsaurusInMemory.Set("//sys/primary_masters", masterAddresses)

	// SaveMasterMonitoringPaths
	h.ytsaurusInMemory.Set("//sys/@cluster_connection/primary_master", map[string]any{
		"cell_id":   "1",
		"addresses": masterAddressesList,
	})
	var masterMonitoringPaths []string
	for _, ms := range masterAddressesList {
		path := "//sys/cluster_masters/" + ms + "/orchid/monitoring/hydra"
		masterMonitoringPaths = append(masterMonitoringPaths, path)
	}
	t.Log("[ Enable Full Update ]")
	ytsaurusResource.Spec.EnableFullUpdate = true
	updateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			status := obj.(*ytv1.Ytsaurus).Status
			return len(status.UpdateStatus.MasterMonitoringPaths) == len(masterMonitoringPaths)
		},
	)

	t.Log("[ Emulate startBuildMasterSnapshots executed ]")
	for _, path := range masterMonitoringPaths {
		h.ytsaurusInMemory.Set(path, map[string]any{
			"read_only":               true,
			"last_snapshot_read_only": true,
		})
	}

	t.Log("[ Emulate master exit read only succeded ]")
	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			status := obj.(*ytv1.Ytsaurus).Status
			return meta.IsStatusConditionTrue(status.UpdateStatus.Conditions, MasterSnapshotsBuiltCondition.Type)
		},
	)
	markJobSucceeded(h, "yt-master-init-job-exit-read-only")

	t.Log("[ Wait for YTsaurus Running status ]")
	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}

func buildMinimalYtsaurus(namespace, name string) ytv1.Ytsaurus {
	remoteYtsaurus := ytv1.Ytsaurus{
		ObjectMeta: v1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: ytv1.YtsaurusSpec{
			CoreImage:        testYtsaurusImage,
			IsManaged:        true,
			EnableFullUpdate: false,
			UseShortNames:    true,

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
				CellTag: 1,
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
	return remoteYtsaurus
}
