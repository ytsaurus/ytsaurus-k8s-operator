package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

func TestYtsaurusClientDependsOnDefaultHTTPProxy(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	other, serving := resource.Spec.HTTPProxies[0], resource.Spec.HTTPProxies[0]
	other.Role, serving.Role = "other", consts.DefaultHTTPProxyRole
	resource.Spec.HTTPProxies = []ytv1.HTTPProxiesSpec{other, serving}
	cm, err := NewComponentManager(t.Context(), apiproxy.NewYtsaurus(&resource, nil, nil, nil), "cluster.local")
	require.NoError(t, err)

	var yc components.Component
	for _, c := range cm.allComponents {
		switch c.GetType() {
		case consts.HttpProxyType:
			c.SetStatus(components.ComponentStatusBlocked("unavailable %s group", c.GetFullName()))
		case consts.YtsaurusClientType:
			yc = c
		}
	}
	require.NotNil(t, yc)
	status, err := yc.Sync(t.Context(), true)
	require.NoError(t, err)
	require.Contains(t, status.Message, "unavailable HttpProxy group")
}
