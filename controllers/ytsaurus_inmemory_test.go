package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

func TestTempGoClient(t *testing.T) {
	ctx := context.Background()

	ytClient, err := ythttp.NewClient(&yt.Config{
		Proxy:                 "localhost:8000",
		Token:                 "password",
		DisableProxyDiscovery: true,
	})
	require.NoError(t, err)

	//var isReadOnly bool
	//err = ytClient.GetNode(context.Background(), ypath.Path("//sys/@hydra_read_only"), isReadOnly, nil)
	//require.False(t, isReadOnly)

	//var tabletCells []string
	//err = ytClient.ListNode(
	//	context.Background(),
	//	ypath.Path("//sys/tablet_cellsx"),
	//	&tabletCells,
	//	nil,
	//)
	//require.NoError(t, err)
	//var smth any
	//err = ytClient.GetNode(context.Background(), ypath.Path("//sys/@nonexistent"), smth, nil)
	//require.Nil(t, smth)

	var tabletCellBundles []components.TabletCellBundleHealth
	err = ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cell_bundles"),
		&tabletCellBundles,
		&yt.ListNodeOptions{Attributes: []string{"health"}})
	require.NoError(t, err)

}

func TestYSON(t *testing.T) {
	type valueWithGenericAttrs struct {
		Attrs map[string]any `yson:",attrs"`
		Value any            `yson:",value"`
	}

	data, err := yson.Marshal([]valueWithGenericAttrs{
		{
			Value: "sys",
			Attrs: map[string]any{"health": "good"},
		},
	})
	require.NoError(t, err)
	require.Empty(t, string(data))
}
