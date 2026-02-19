package components

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	"go.ytsaurus.tech/yt/go/yterrors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	hydraPath = "orchid/monitoring/hydra"
)

type internalYtsaurusClient interface {
	Component
	GetYtClient() yt.Client
}

type YtsaurusClient struct {
	component

	cfgen     *ytconfig.Generator
	httpProxy Component

	getAllComponents func() []Component
	configOverrides  *resources.ConfigMap
	cypressPatch     *resources.ConfigMap

	initUserJob *InitJob

	secret   *resources.StringSecret
	ytClient yt.Client
}

func NewYtsaurusClient(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	httpProxy Component,
	getAllComponents func() []Component,

) *YtsaurusClient {
	l := cfgen.GetComponentLabeller(consts.YtsaurusClientType, "")
	resource := ytsaurus.GetResource()

	var configOverrides *resources.ConfigMap
	if overrides := resource.Spec.ConfigOverrides; overrides != nil {
		configOverrides = resources.NewConfigMap(overrides.Name, l, ytsaurus)
	}

	return &YtsaurusClient{
		component:        newComponent(l, ytsaurus),
		cfgen:            cfgen,
		httpProxy:        httpProxy,
		getAllComponents: getAllComponents,
		configOverrides:  configOverrides,
		cypressPatch: resources.NewConfigMap(
			l.GetCypressPatchConfigMapName(),
			l,
			ytsaurus,
		),
		initUserJob: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"user",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&ytv1.InstanceSpec{},
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
	}
}

func (yc *YtsaurusClient) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		yc.secret,
		yc.initUserJob,
		yc.httpProxy,
		yc.configOverrides,
		yc.cypressPatch,
	)
}

func (yc *YtsaurusClient) Exists() bool {
	return resources.Exists(yc.secret, yc.initUserJob, yc.httpProxy, yc.cypressPatch)
}

func (yc *YtsaurusClient) createInitUserScript() string {
	token, _ := yc.secret.GetValue(consts.TokenSecretKey)
	initJob := initJobWithNativeDriverPrologue()
	return initJob + "\n" + strings.Join(createUserCommand(consts.YtsaurusOperatorUserName, "", token, true), "\n")
}

type TabletCellBundleHealth struct {
	Name   string `yson:",value" json:"name"`
	Health string `yson:"health,attr" json:"health"`
}

type MasterInfo struct {
	CellID    string   `yson:"cell_id" json:"cellId"`
	Addresses []string `yson:"addresses" json:"addresses"`
}

func getReadOnlyGetOptions() *yt.GetNodeOptions {
	return &yt.GetNodeOptions{
		TransactionOptions: &yt.TransactionOptions{
			SuppressUpstreamSync:               true,
			SuppressTransactionCoordinatorSync: true,
		},
	}
}

type MasterState string

const (
	MasterStateLeading   MasterState = "leading"
	MasterStateFollowing MasterState = "following"
)

type MasterHydra struct {
	ReadOnly             bool        `yson:"read_only"`
	LastSnapshotReadOnly bool        `yson:"last_snapshot_read_only"`
	Active               bool        `yson:"active"`
	State                MasterState `yson:"state"`
}

func (yc *YtsaurusClient) getAllMasters(ctx context.Context) ([]MasterInfo, error) {
	var primaryMaster MasterInfo
	err := yc.ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/primary_master"), &primaryMaster, getReadOnlyGetOptions())
	if err != nil {
		return nil, err
	}

	var secondaryMasters []MasterInfo
	if len(yc.ytsaurus.GetResource().Spec.SecondaryMasters) > 0 {
		err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/secondary_masters"), &secondaryMasters, getReadOnlyGetOptions())
		if err != nil {
			return nil, err
		}
	}

	return append(secondaryMasters, primaryMaster), nil
}

// shouldSkipCypressOperations returns true when no alive masters are expected.
func (yc *YtsaurusClient) shouldSkipCypressOperations() bool {
	resource := yc.ytsaurus.GetResource()
	return resource.Spec.EphemeralCluster && ptr.Deref(resource.Spec.PrimaryMasters.MinReadyInstanceCount, 1) == 0
}

func (yc *YtsaurusClient) handleUpdatingState(ctx context.Context) (ComponentStatus, error) {
	var err error

	switch yc.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStatePossibilityCheck:
		// FIXME(khlebnikov): Remove redundant inverted condition and refactor.
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) &&
			!yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {
			ok, msg, err := yc.HandlePossibilityCheck(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !ok {
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionNoPossibility,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: msg,
				})
				return SimpleStatus(SyncStatusUpdating), nil
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionHasPossibility,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: msg,
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSafeModeEnabled:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {
			err := yc.EnableSafeMode(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSafeModeEnabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Safe mode was enabled",
			})

			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsSaving:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {
			tabletCellBundles, err := yc.GetTabletCells(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles = tabletCellBundles

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells were saved",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {
			err = yc.RemoveTabletCells(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRemovingStarted,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells removing was started",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			removed, err := yc.AreTabletCellsRemoved(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !removed {
				return SimpleStatus(SyncStatusUpdating), nil
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRemoved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells were removed",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForImaginaryChunksAbsence:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionRealChunksAttributeEnabled) {
			if err := yc.ensureRealChunkLocationsEnabled(ctx); err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionRealChunksAttributeEnabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Real chunk locations attribute is set to true",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionDataNodesWithImaginaryChunksAbsent) {
			haveImaginaryChunks, err := yc.areActiveDataNodesWithImaginaryChunksExist(ctx)
			if !haveImaginaryChunks {
				// Either no imaginary chunks been found
				// or imaginary chunk nodes pods were removed
				// and master doesn't have online data nodes with imaginary chunks.
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionDataNodesWithImaginaryChunksAbsent,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "No active data nodes with imaginary chunks found",
				})
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionDataNodesNeedPodsRemoval) {
				// Requesting data nodes to remove pods.
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionDataNodesNeedPodsRemoval,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Some data nodes have imaginary chunks and need to be restarted",
				})
			}
			// Waiting for data nodes to remove pods and areOnlineDataNodesWithImaginaryChunksExist to return false
			// in the next reconciliations.
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			dnds, err := yc.getDataNodesInfo(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
			log.FromContext(ctx).Info("data nodes before snapshots building", "dataNodes", dnds)
			if err := yc.BuildMasterSnapshots(ctx); err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSnaphotsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Master snapshots were built",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsRecovery:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {
			err = yc.RecoverTableCells(ctx, yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRecovered,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells recovered",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSafeModeDisabled:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {
			err := yc.DisableSafeMode(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSafeModeDisabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Safe mode disabled",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForCypressPatch:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionCypressPatchApplied) {
			if err := yc.SyncCypressPatch(ctx); err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionCypressPatchApplied,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Cypress patches applied",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}
	}

	return SimpleStatus(SyncStatusUpdating), err
}

func (yc *YtsaurusClient) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	hpStatus, err := yc.httpProxy.Status(ctx)
	if err != nil {
		return hpStatus, err
	}
	if !hpStatus.IsRunning() {
		return ComponentStatusBlockedBy(yc.httpProxy.GetFullName()), err
	}

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := yc.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = yc.secret.Sync(ctx)
		}
		return ComponentStatusWaitingFor(yc.secret.Name()), err
	}

	if !dry {
		yc.initUserJob.SetInitScript(yc.createInitUserScript())
	}
	status, err := yc.initUserJob.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if yc.ytClient == nil {
		token, _ := yc.secret.GetValue(consts.TokenSecretKey)
		timeout := time.Second * 10
		proxy, ok := os.LookupEnv("YTOP_PROXY")
		disableProxyDiscovery := true
		if !ok {
			proxy = yc.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
			disableProxyDiscovery = false
		}
		yc.ytClient, err = ythttp.NewClient(&yt.Config{
			Proxy:                 proxy,
			Token:                 token,
			LightRequestTimeout:   &timeout,
			DisableProxyDiscovery: disableProxyDiscovery,
		})

		if err != nil {
			return ComponentStatusWaitingFor("ytClient init"), err
		}
	}

	if yc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if yc.ytsaurus.GetUpdateState() == ytv1.UpdateStateImpossibleToStart {
			return ComponentStatusReady(), err
		}
		if dry {
			return SimpleStatus(SyncStatusUpdating), err
		}
		return yc.handleUpdatingState(ctx)
	}

	if status := yc.NeedSyncCypressPatch(); status.SyncStatus != SyncStatusReady {
		if !dry {
			if err := yc.SyncCypressPatch(ctx); err != nil {
				return ComponentStatusPending("error"), err
			}
		}
		return status, nil
	}

	return ComponentStatusReady(), err
}

func (yc *YtsaurusClient) NeedSyncCypressPatch() ComponentStatus {
	if !yc.cypressPatch.Exists() {
		return ComponentStatusPending(fmt.Sprintf("Need to create %s", yc.cypressPatch.Name()))
	}

	if yc.configOverrides != nil && yc.configOverrides.Exists() {
		patchOverridesVersion := yc.cypressPatch.OldObject().Annotations[consts.ConfigOverridesVersionAnnotationName]
		overridesVersion := yc.configOverrides.OldObject().ResourceVersion
		if overridesVersion != patchOverridesVersion {
			return ComponentStatusPending(fmt.Sprintf("Need to update overrides in %s", yc.cypressPatch.Name()))
		}
	}

	if !yc.ytsaurus.IsStatusConditionTrue(consts.ConditionCypressPatchApplied) {
		if yc.shouldSkipCypressOperations() {
			return ComponentStatusReadyAfter("Cypress patch applying is skipped")
		}
		return ComponentStatusPending("Need to apply cypress patch")
	}

	return ComponentStatusReadyAfter("Cypress patch is up to date and applied")
}

func (yc *YtsaurusClient) Status(ctx context.Context) (ComponentStatus, error) {
	return yc.doSync(ctx, true)
}

func (yc *YtsaurusClient) NeedSync() bool {
	return false
}

func (yc *YtsaurusClient) NeedUpdate() ComponentStatus {
	return ComponentStatus{}
}

func (yc *YtsaurusClient) Sync(ctx context.Context) error {
	_, err := yc.doSync(ctx, false)
	return err
}

func (yc *YtsaurusClient) GetYtClient() yt.Client {
	return yc.ytClient
}

func (yc *YtsaurusClient) HandlePossibilityCheck(ctx context.Context) (ok bool, msg string, err error) {
	// Check tablet cell bundles.
	notGoodBundles, err := GetNotGoodTabletCellBundles(ctx, yc.ytClient)
	if err != nil {
		return false, "", err
	}

	if len(notGoodBundles) > 0 {
		msg = fmt.Sprintf("Tablet cell bundles (%v) aren't in 'good' health", notGoodBundles)
		return false, msg, nil
	}

	// Check LVC.
	lvcCount := 0
	err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/lost_vital_chunks/@count"), &lvcCount, nil)
	if err != nil {
		return false, "", err
	}

	if lvcCount > 0 {
		msg = fmt.Sprintf("There are lost vital chunks: %v", lvcCount)
		return false, msg, nil
	}

	// Check QMC.
	qmcCount := 0
	err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/quorum_missing_chunks/@count"), &qmcCount, nil)
	if err != nil {
		return false, "", err
	}

	if qmcCount > 0 {
		msg = fmt.Sprintf("There are quorum missing chunks: %v", qmcCount)
		return false, msg, nil
	}

	// Check is masters quorum healthy
	msg, err = yc.checkMastersQuorumHealth(ctx)
	if err != nil {
		return false, "", err
	}
	if msg != "" {
		return false, msg, nil
	}

	return true, "Update is possible", nil
}

// Safe mode actions.

func (yc *YtsaurusClient) EnableSafeMode(ctx context.Context) error {
	return yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), true, nil)
}
func (yc *YtsaurusClient) DisableSafeMode(ctx context.Context) error {
	return yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), false, nil)
}

// Cypress patches actions.

func (yc *YtsaurusClient) GetCypressPatch() ypatch.PatchSet {
	return ypatch.PatchSet{
		// Copy cluster connection into list of cluster after applying all other patches.
		"<>//sys/clusters": {
			ypatch.Copy(
				ypath.Path("/"+yc.labeller.GetClusterName()),
				"//sys/@cluster_connection",
			),
		},
	}
}

func (yc *YtsaurusClient) BuildCypressPatch(ctx context.Context) (*corev1.ConfigMap, ypatch.PatchSet, error) {
	logger := log.FromContext(ctx)

	var err error

	description := &strings.Builder{}
	currentPatch := ypatch.PatchSet{}
	pendingPatch := ypatch.PatchSet{}

	cp := yc.cypressPatch.Build()

	if yc.configOverrides != nil && yc.configOverrides.Exists() {
		overridesVersion := yc.configOverrides.OldObject().ResourceVersion
		metav1.SetMetaDataAnnotation(
			&cp.ObjectMeta,
			consts.ConfigOverridesVersionAnnotationName,
			overridesVersion,
		)
		fmt.Fprintf(description, "overrides-version: %v\n", overridesVersion)
	}

	for _, component := range yc.getAllComponents() {
		componentPatch := component.GetCypressPatch()

		// File names for component patches.
		patchFileName := component.GetLabeller().GetCypressPatchFileName(consts.CypressPatchFileName)
		pendingFileName := component.GetLabeller().GetCypressPatchFileName(consts.PendingCypressPatchFileName)
		previousFileName := component.GetLabeller().GetCypressPatchFileName(consts.PreviousCypressPatchFileName)

		// Apply component patch override.
		if yc.configOverrides != nil && yc.configOverrides.Exists() {
			if data, found := yc.configOverrides.OldObject().Data[patchFileName]; found {
				logger.Info("Add cypress patch override", "name", patchFileName)
				override := ypatch.PatchSet{}
				if err = yson.Unmarshal([]byte(data), &override); err != nil {
					return nil, nil, fmt.Errorf("cannot parse cypress patch override %s: %w", patchFileName, err)
				}
				if componentPatch == nil {
					componentPatch = override
				} else {
					componentPatch.AddPatchSet(override)
				}
			}
		}

		var componentPatchData []byte
		if componentPatchData, err = yson.MarshalFormat(componentPatch, yson.FormatPretty); err != nil {
			return nil, nil, fmt.Errorf("cannot format component cypress patch %s: %w", patchFileName, err)
		}

		if !yc.cypressPatch.Exists() || IsUpdatingComponent(yc.ytsaurus, component) ||
			yc.ytsaurus.GetClusterState() == ytv1.ClusterStateInitializing {
			// Include empty patches for clarity.
			cp.Data[patchFileName] = string(componentPatchData)
			currentPatch.AddPatchSet(componentPatch)

			// Update previous patch if something has changed.
			previousData := yc.cypressPatch.OldObject().Data[patchFileName]
			if string(componentPatchData) == previousData {
				previousData = yc.cypressPatch.OldObject().Data[previousFileName]
			}
			if previousData != "" {
				cp.Data[previousFileName] = previousData
			}
		} else {
			// Use saved patch if component is not updating.
			var savedPatchData string
			if data, found := yc.cypressPatch.OldObject().Data[patchFileName]; found {
				savedPatchData = data
				savedPatch := ypatch.PatchSet{}
				if err := yson.Unmarshal([]byte(data), &savedPatch); err != nil {
					return nil, nil, fmt.Errorf("cannot parse current cypress patch %s: %w", patchFileName, err)
				}
				currentPatch.AddPatchSet(savedPatch)
				cp.Data[patchFileName] = savedPatchData
			}

			// Include pending patch when something going to be updated.
			if string(componentPatchData) != savedPatchData {
				cp.Data[pendingFileName] = string(componentPatchData)
				pendingPatch.AddPatchSet(componentPatch)
				fmt.Fprintf(description, "pending: %v\n", component.GetLabeller().GetFullComponentLabel())
			}

			// Preserve previous patch when exists.
			if data, found := yc.cypressPatch.OldObject().Data[previousFileName]; found {
				cp.Data[previousFileName] = data
			}
		}
	}

	// Apply global cypress patch override.
	if yc.configOverrides != nil && yc.configOverrides.Exists() {
		if data, found := yc.configOverrides.OldObject().Data[consts.CypressPatchFileName]; found {
			logger.Info("Adding cypress patch override", "name", consts.CypressPatchFileName)
			patch := ypatch.PatchSet{}
			if err := yson.Unmarshal([]byte(data), &patch); err != nil {
				return nil, nil, fmt.Errorf("cannot parse cypress patch override %s: %w", consts.CypressPatchFileName, err)
			}
			currentPatch.AddPatchSet(patch)
		}
	}

	{
		currentPatchData, err := yson.MarshalFormat(currentPatch, yson.FormatPretty)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot format current cypress patch %s: %w", consts.CypressPatchFileName, err)
		}

		cp.Data[consts.CypressPatchFileName] = string(currentPatchData)

		if len(pendingPatch) != 0 {
			pendingPatchData, err := yson.MarshalFormat(pendingPatch, yson.FormatPretty)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot format pending cypress patch %s: %w", consts.PendingCypressPatchFileName, err)
			}
			cp.Data[consts.PendingCypressPatchFileName] = string(pendingPatchData)
		}

		previousPatchData := yc.cypressPatch.OldObject().Data[consts.CypressPatchFileName]
		if string(currentPatchData) == previousPatchData {
			previousPatchData = yc.cypressPatch.OldObject().Data[consts.PreviousCypressPatchFileName]
		}
		if previousPatchData != "" {
			cp.Data[consts.PreviousCypressPatchFileName] = previousPatchData
		}
	}

	metav1.SetMetaDataAnnotation(&cp.ObjectMeta, consts.DescriptionAnnotationName, description.String())

	return cp, currentPatch, nil
}

func (yc *YtsaurusClient) SyncCypressPatch(ctx context.Context) error {
	logger := log.FromContext(ctx)
	cp, patch, err := yc.BuildCypressPatch(ctx)
	if err != nil {
		logger.Error(err, "Failed to build cypress patch")
		yc.ytsaurus.RecordWarning("CypressPatch", fmt.Sprintf("Failed to build cypress patch: %v", err))
		yc.ytsaurus.SetStatusCondition(metav1.Condition{
			LastTransitionTime: metav1.Now(),
			Type:               consts.ConditionCypressPatchApplied,
			Status:             metav1.ConditionFalse,
			Reason:             "PatchBuildFailed",
			Message:            fmt.Sprintf("Error: %v", err),
		})
		return nil
	}
	if err := yc.cypressPatch.Sync(ctx); err != nil {
		return err
	}
	if yc.shouldSkipCypressOperations() {
		logger.Info("Skipping cypress patch apply in test")
		yc.ytsaurus.RecordNormal("CypressPatch", "Skip patch apply in test")
		return nil
	}
	cypressPatchTarget := ypatch.CypressPatchTarget{
		Client: yc.ytClient,
	}
	err = cypressPatchTarget.ApplyPatchSet(ctx, "", patch)
	if err == nil {
		logger.Info("Cypress patch applied")
		yc.ytsaurus.RecordNormal("CypressPatch", "patch applied")
		yc.ytsaurus.SetStatusCondition(metav1.Condition{
			LastTransitionTime: metav1.Now(),
			Type:               consts.ConditionCypressPatchApplied,
			Status:             metav1.ConditionTrue,
			Reason:             "PatchApplied",
			Message:            fmt.Sprintf("Description: %v", cp.Annotations[consts.DescriptionAnnotationName]),
		})
	} else {
		logger.Error(err, "Failed to apply cypress patch")
		yc.ytsaurus.RecordWarning("CypressPatch", fmt.Sprintf("Failed to apply cypress patch: %v", err))
		yc.ytsaurus.SetStatusCondition(metav1.Condition{
			LastTransitionTime: metav1.Now(),
			Type:               consts.ConditionCypressPatchApplied,
			Status:             metav1.ConditionFalse,
			Reason:             "PatchApplyFailed",
			Message:            fmt.Sprintf("Error: %v", err),
		})
	}
	return nil
}

// Tablet cells actions.

func (yc *YtsaurusClient) GetTabletCells(ctx context.Context) ([]ytv1.TabletCellBundleInfo, error) {
	var tabletCellBundles []ytv1.TabletCellBundleInfo
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cell_bundles"),
		&tabletCellBundles,
		&yt.ListNodeOptions{Attributes: []string{"tablet_cell_count"}},
	)

	if err != nil {
		return nil, err
	}
	return tabletCellBundles, nil
}
func (yc *YtsaurusClient) RemoveTabletCells(ctx context.Context) error {
	var tabletCells []string
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cells"),
		&tabletCells,
		nil)

	if err != nil {
		return err
	}

	for _, tabletCell := range tabletCells {
		err = yc.ytClient.RemoveNode(
			ctx,
			ypath.Path(fmt.Sprintf("//sys/tablet_cells/%s", tabletCell)),
			nil)
		if err != nil {
			return err
		}
	}
	return nil
}
func (yc *YtsaurusClient) AreTabletCellsRemoved(ctx context.Context) (bool, error) {
	var tabletCells []string
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cells"),
		&tabletCells,
		nil)

	if err != nil {
		return false, err
	}

	if len(tabletCells) != 0 {
		return false, err
	}
	return true, nil
}
func (yc *YtsaurusClient) RecoverTableCells(ctx context.Context, bundles []ytv1.TabletCellBundleInfo) error {
	for _, bundle := range bundles {
		err := CreateTabletCells(ctx, yc.ytClient, bundle.Name, bundle.TabletCellCount)
		if err != nil {
			return err
		}
	}
	return nil
}

// Master actions.

type MastersWithMaintenance struct {
	Address     string `yson:",value"`
	Maintenance bool   `yson:"maintenance,attr"`
}

func (yc *YtsaurusClient) checkMastersQuorumHealth(ctx context.Context) (string, error) {
	primaryMastersWithMaintenance := make([]MastersWithMaintenance, 0)
	cypressPath := consts.ComponentCypressPath(consts.MasterType)

	err := yc.ytClient.ListNode(ctx, ypath.Path(cypressPath), &primaryMastersWithMaintenance, &yt.ListNodeOptions{
		Attributes: []string{"maintenance"}})
	if err != nil {
		return "", err
	}

	leadingPrimaryMasterCount := 0
	followingPrimaryMasterCount := 0

	for _, primaryMaster := range primaryMastersWithMaintenance {
		var hydra MasterHydra
		err = yc.ytClient.GetNode(
			ctx,
			ypath.Path(fmt.Sprintf("%v/%v/%v", cypressPath, primaryMaster.Address, hydraPath)),
			&hydra,
			nil)
		if err != nil {
			return "", err
		}

		if !hydra.Active {
			msg := fmt.Sprintf("There is a non-active master: %v", primaryMaster.Address)
			return msg, nil
		}

		if primaryMaster.Maintenance {
			msg := fmt.Sprintf("There is a master in maintenance: %v", primaryMaster.Address)
			return msg, nil
		}

		switch hydra.State {
		case MasterStateLeading:
			leadingPrimaryMasterCount += 1
		case MasterStateFollowing:
			followingPrimaryMasterCount += 1
		}
	}

	if !(leadingPrimaryMasterCount == 1 && followingPrimaryMasterCount+1 == len(primaryMastersWithMaintenance)) {
		msg := fmt.Sprintf("Quorum health check failed: leading=%d, following=%d, total=%d",
			leadingPrimaryMasterCount, followingPrimaryMasterCount, len(primaryMastersWithMaintenance))
		return msg, nil
	}
	return "", nil
}

func (yc *YtsaurusClient) GetMasterMonitoringPaths(ctx context.Context) ([]string, error) {
	var monitoringPaths []string
	mastersInfo, err := yc.getAllMasters(ctx)
	if err != nil {
		return nil, err
	}

	for _, masterInfo := range mastersInfo {
		for _, address := range masterInfo.Addresses {
			monitoringPath := fmt.Sprintf("//sys/cluster_masters/%s/%v", address, hydraPath)
			monitoringPaths = append(monitoringPaths, monitoringPath)
		}
	}
	return monitoringPaths, nil
}
func (yc *YtsaurusClient) BuildMasterSnapshots(ctx context.Context) error {
	_, err := yc.ytClient.BuildMasterSnapshots(ctx, &yt.BuildMasterSnapshotsOptions{
		WaitForSnapshotCompletion: ptr.To(true),
		SetReadOnly:               ptr.To(true),
	})

	return err
}
func (yc *YtsaurusClient) AreMasterSnapshotsBuilt(ctx context.Context, monitoringPaths []string) (bool, error) {
	for _, monitoringPath := range monitoringPaths {
		var masterHydra MasterHydra
		err := yc.ytClient.GetNode(ctx, ypath.Path(monitoringPath), &masterHydra, getReadOnlyGetOptions())
		if err != nil {
			return false, err
		}

		if !masterHydra.LastSnapshotReadOnly {
			return false, nil
		}
	}
	return true, nil
}

func (yc *YtsaurusClient) ensureRealChunkLocationsEnabled(ctx context.Context) error {
	logger := log.FromContext(ctx)

	realChunkLocationPath := "//sys/@config/node_tracker/enable_real_chunk_locations"
	var isChunkLocationsEnabled bool
	err := yc.ytClient.GetNode(ctx, ypath.Path(realChunkLocationPath), &isChunkLocationsEnabled, nil)
	if err != nil {
		// FIXME(khlebnikov): Check master reigh.
		if yterrors.ContainsResolveError(err) {
			logger.Info(fmt.Sprintf("%s doesn't exist, this is the case for 24.2+ versions, nothing to do", realChunkLocationPath))
			return nil
		}
		return fmt.Errorf("failed to get %s: %w", realChunkLocationPath, err)
	}

	logger.Info(fmt.Sprintf("enable_real_chunk_locations is %t", isChunkLocationsEnabled))

	if isChunkLocationsEnabled {
		return nil
	}

	err = yc.ytClient.SetNode(ctx, ypath.Path(realChunkLocationPath), true, nil)
	if err != nil {
		return fmt.Errorf("failed to set %s: %w", realChunkLocationPath, err)
	}
	logger.Info("enable_real_chunk_locations is set to true")
	return nil
}

type DataNodeMeta struct {
	Name               string `yson:",value"`
	UseImaginaryChunks bool   `yson:"use_imaginary_chunk_locations,attr"`
	State              string `yson:"state,attr"`
}

func (yc *YtsaurusClient) getDataNodesInfo(ctx context.Context) ([]DataNodeMeta, error) {
	var dndMeta []DataNodeMeta
	err := yc.ytClient.ListNode(ctx, ypath.Path(consts.ComponentCypressPath(consts.DataNodeType)), &dndMeta, &yt.ListNodeOptions{
		Attributes: []string{
			"use_imaginary_chunk_locations",
			"state",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list data_nodes: %w", err)
	}
	return dndMeta, nil
}

func (yc *YtsaurusClient) areActiveDataNodesWithImaginaryChunksExist(ctx context.Context) (bool, error) {
	dndMeta, err := yc.getDataNodesInfo(ctx)
	if err != nil {
		return false, err
	}

	var activeNodesWithImaginaryChunks []DataNodeMeta
	for _, dnd := range dndMeta {
		if dnd.UseImaginaryChunks && dnd.State != "offline" {
			activeNodesWithImaginaryChunks = append(activeNodesWithImaginaryChunks, dnd)
		}
	}
	logger := log.FromContext(ctx)
	if len(activeNodesWithImaginaryChunks) == 0 {
		logger.Info("There are 0 active nodes with imaginary chunk locations")
		return false, nil
	}
	logger.Info(
		fmt.Sprintf("There are %d active nodes with imaginary chunk locations", len(activeNodesWithImaginaryChunks)),
		"sample", activeNodesWithImaginaryChunks[:1],
	)
	return true, nil
}
