package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
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

	ok, err := ytClient.NodeExists(ctx, ypath.Path("//sys/@enable_safe_mode"), nil)
	require.NoError(t, err)
	require.True(t, ok)
}
