package components

import (
	"context"
	"fmt"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

func CreateUserCommand(ctx context.Context, ytClient yt.Client, userName, token string, isSuperuser bool) error {
	var err error
	_, err = ytClient.CreateNode(ctx, ypath.Path(""), yt.NodeUser, &yt.CreateNodeOptions{
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
