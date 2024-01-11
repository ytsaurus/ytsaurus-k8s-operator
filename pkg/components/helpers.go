package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/library/go/ptr"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

func CreateTabletCells(ctx context.Context, ytClient yt.Client, bundle string, tabletCellCount int) error {
	logger := log.FromContext(ctx)

	var initTabletCellCount int

	if err := ytClient.GetNode(
		ctx,
		ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", bundle)),
		&initTabletCellCount,
		nil); err != nil {

		logger.Error(err, "Getting table_cell_count failed")
		return err
	}

	for i := initTabletCellCount; i < tabletCellCount; i += 1 {
		_, err := ytClient.CreateObject(ctx, "tablet_cell", &yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"tablet_cell_bundle": bundle,
			},
		})

		if err != nil {
			logger.Error(err, "Creating tablet_cell failed")
			return err
		}
	}
	return nil
}

func GetNotGoodTabletCellBundles(ctx context.Context, ytClient yt.Client) ([]string, error) {
	var tabletCellBundles []TabletCellBundleHealth
	err := ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cell_bundles"),
		&tabletCellBundles,
		&yt.ListNodeOptions{Attributes: []string{"health"}})

	if err != nil {
		return nil, err
	}

	notGoodBundles := make([]string, 0)
	for _, bundle := range tabletCellBundles {
		if bundle.Health != "good" {
			notGoodBundles = append(notGoodBundles, bundle.Name)
		}
	}

	return notGoodBundles, err
}

func CreateUser(ctx context.Context, ytClient yt.Client, userName, token string, isSuperuser bool) error {
	var err error
	_, err = ytClient.CreateObject(ctx, yt.NodeUser, &yt.CreateObjectOptions{
		IgnoreExisting: true,
		Attributes: map[string]interface{}{
			"name": userName,
		}})
	if err != nil {
		return err
	}

	if token != "" {
		tokenHash := sha256String(token)
		tokenPath := fmt.Sprintf("//sys/cypress_tokens/%s", tokenHash)

		_, err := ytClient.CreateNode(
			ctx,
			ypath.Path(tokenPath),
			yt.NodeMap,
			&yt.CreateNodeOptions{
				IgnoreExisting: true,
			},
		)
		if err != nil {
			return err
		}

		err = ytClient.SetNode(ctx, ypath.Path(tokenPath).Attr("user"), userName, nil)
		if err != nil {
			return err
		}
	}

	if isSuperuser {
		_ = ytClient.AddMember(ctx, "superusers", userName, nil)
	}

	return err
}

func IsUpdatingComponent(ytsaurus *apiproxy.Ytsaurus, component Component) bool {
	componentNames := ytsaurus.GetLocalUpdatingComponents()
	return (componentNames == nil && component.IsUpdatable()) || slices.Contains(componentNames, component.GetName())
}

func handleUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *localComponent,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	var err error

	if IsUpdatingComponent(ytsaurus, cmp) {
		if ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			if !dry {
				err = removePods(ctx, server, cmpBase)
			}
			return ptr.T(WaitingStatus(SyncStatusUpdating, "pods removal")), err
		}

		if ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
			return ptr.T(NewComponentStatus(SyncStatusReady, "Nothing to do now")), err
		}
	} else {
		return ptr.T(NewComponentStatus(SyncStatusReady, "Not updating component")), err
	}
	return nil, err
}
