package apiproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytcluster"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

var componentTestCases = []struct {
	component consts.ComponentType
	newObject func(metav1.ObjectMeta, ytv1.YtsaurusReference) ytcluster.ComponentObject
}{
	{consts.ExecNodeType, func(meta metav1.ObjectMeta, parent ytv1.YtsaurusReference) ytcluster.ComponentObject {
		return &ytv1.YtsaurusExecNodes{ObjectMeta: meta, Spec: ytv1.YtsaurusExecNodesSpec{
			Ytsaurus:      parent,
			ExecNodesSpec: ytv1.ExecNodesSpec{Name: meta.Name},
		}}
	}},
	{consts.DataNodeType, func(meta metav1.ObjectMeta, parent ytv1.YtsaurusReference) ytcluster.ComponentObject {
		return &ytv1.YtsaurusDataNodes{ObjectMeta: meta, Spec: ytv1.YtsaurusDataNodesSpec{
			Ytsaurus:      parent,
			DataNodesSpec: ytv1.DataNodesSpec{Name: meta.Name},
		}}
	}},
	{consts.TabletNodeType, func(meta metav1.ObjectMeta, parent ytv1.YtsaurusReference) ytcluster.ComponentObject {
		return &ytv1.YtsaurusTabletNodes{ObjectMeta: meta, Spec: ytv1.YtsaurusTabletNodesSpec{
			Ytsaurus:        parent,
			TabletNodesSpec: ytv1.TabletNodesSpec{Name: meta.Name},
		}}
	}},
	{consts.HttpProxyType, func(meta metav1.ObjectMeta, parent ytv1.YtsaurusReference) ytcluster.ComponentObject {
		return &ytv1.YtsaurusHTTPProxies{ObjectMeta: meta, Spec: ytv1.YtsaurusHTTPProxiesSpec{
			Ytsaurus:        parent,
			HTTPProxiesSpec: ytv1.HTTPProxiesSpec{Role: meta.Name},
		}}
	}},
	{consts.RpcProxyType, func(meta metav1.ObjectMeta, parent ytv1.YtsaurusReference) ytcluster.ComponentObject {
		return &ytv1.YtsaurusRPCProxies{ObjectMeta: meta, Spec: ytv1.YtsaurusRPCProxiesSpec{
			Ytsaurus:       parent,
			RPCProxiesSpec: ytv1.RPCProxiesSpec{Role: meta.Name},
		}}
	}},
}

func TestGetClusterResources(t *testing.T) {
	for _, tc := range componentTestCases {
		t.Run(string(tc.component), func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, ytv1.AddToScheme(scheme))
			resource := &ytv1.Ytsaurus{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "namespace"},
				Spec: ytv1.YtsaurusSpec{
					ExecNodes:   []ytv1.ExecNodesSpec{{Name: "inline"}},
					DataNodes:   []ytv1.DataNodesSpec{{Name: "inline"}},
					TabletNodes: []ytv1.TabletNodesSpec{{Name: "inline"}},
					HTTPProxies: []ytv1.HTTPProxiesSpec{{Role: "inline"}},
					RPCProxies:  []ytv1.RPCProxiesSpec{{Role: "inline"}},
				},
			}
			original := resource.DeepCopy()
			nodes := func(name, namespace, clusterName string) client.Object {
				return tc.newObject(metav1.ObjectMeta{Name: name, Namespace: namespace}, ytv1.YtsaurusReference{Name: clusterName})
			}
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme)
			clientBuilder.WithObjects(
				nodes("attached-a", resource.Namespace, resource.Name),
				nodes("attached-b", resource.Namespace, resource.Name),
				nodes("other-cluster", resource.Namespace, "other"),
				nodes("other-namespace", "other", resource.Name),
			)

			proxy := NewYtsaurus(resource, clientBuilder.Build(), nil, scheme)
			require.NoError(t, proxy.FetchClusterComponents(t.Context()))
			generator := ytconfig.NewGenerator(proxy.Cluster, "cluster.local")
			var names []string
			for _, component := range generator.GetComponents() {
				if component.Type == tc.component {
					names = append(names, component.Name)
				}
			}
			require.ElementsMatch(t, []string{"inline", "attached-a", "attached-b"}, names)
			require.Same(t, &resource.Spec.ExecNodes[0], proxy.ExecNodes[0])
			require.Same(t, &resource.Spec.DataNodes[0], proxy.DataNodes[0])
			require.Same(t, &resource.Spec.TabletNodes[0], proxy.TabletNodes[0])
			require.Same(t, &resource.Spec.HTTPProxies[0], proxy.HTTPProxies[0])
			require.Same(t, &resource.Spec.RPCProxies[0], proxy.RPCProxies[0])
			require.Equal(t, original.Spec, resource.Spec)
		})
	}
}

func TestUpdateComponentStatus(t *testing.T) {
	for _, tc := range componentTestCases {
		t.Run(string(tc.component), func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, ytv1.AddToScheme(scheme))
			cluster := &ytv1.Ytsaurus{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "namespace", Generation: 10},
			}
			parent := ytv1.YtsaurusReference{Name: cluster.Name}
			resource := tc.newObject(metav1.ObjectMeta{Name: "attached", Namespace: cluster.Namespace, Generation: 7}, parent)
			switch resource := resource.(type) {
			case *ytv1.YtsaurusExecNodes:
				resource.Spec.InstanceCount = 3
			case *ytv1.YtsaurusDataNodes:
				resource.Spec.InstanceCount = 3
			case *ytv1.YtsaurusTabletNodes:
				resource.Spec.InstanceCount = 3
			case *ytv1.YtsaurusHTTPProxies:
				resource.Spec.InstanceCount = 3
			case *ytv1.YtsaurusRPCProxies:
				resource.Spec.InstanceCount = 3
			}
			component := ytv1.Component{Type: tc.component, Name: "attached"}
			other := tc.newObject(metav1.ObjectMeta{Name: "attached", Namespace: "other"}, parent)
			builder := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(resource, other).WithObjects(resource, other)
			k8sClient := builder.Build()
			proxy := NewYtsaurus(cluster, k8sClient, nil, scheme)
			require.NoError(t, proxy.FetchClusterComponents(t.Context()))

			for _, state := range []ytv1.ComponentState{
				ytv1.ComponentStatePending, ytv1.ComponentStateBlocked, ytv1.ComponentStateStarted,
				ytv1.ComponentStateReady, ytv1.ComponentStateNeedUpdate, ytv1.ComponentStateUpdating,
			} {
				status := ytv1.ComponentStatus{State: state, Message: "Component is " + string(state)}
				require.NoError(t, proxy.UpdateClusterComponentStatus(t.Context(), component, status))
				require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(resource), resource))
				require.Equal(t, status, resource.GetComponentStatus())
				require.Equal(t, int64(7), resource.GetStatusObservedGeneration())
				version := resource.GetResourceVersion()
				require.NoError(t, proxy.UpdateClusterComponentStatus(t.Context(), component, status))
				require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(resource), resource))
				require.Equal(t, version, resource.GetResourceVersion(), "unchanged status must not trigger another write")
			}

			// A new resource generation must be observed even if its state and message stay the same.
			resource.SetGeneration(8)
			require.NoError(t, k8sClient.Update(t.Context(), resource))
			proxy = NewYtsaurus(cluster, k8sClient, nil, scheme)
			require.NoError(t, proxy.FetchClusterComponents(t.Context()))
			require.NoError(t, proxy.UpdateClusterComponentStatus(t.Context(), component, resource.GetComponentStatus()))
			require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(resource), resource))
			require.Equal(t, int64(8), resource.GetStatusObservedGeneration())
			require.NoError(t, k8sClient.Get(t.Context(), client.ObjectKeyFromObject(other), other))
			require.Empty(t, other.GetComponentStatus(), "components in other namespaces must not be updated")

			require.NoError(t, proxy.UpdateClusterComponentStatus(t.Context(), ytv1.Component{Type: tc.component, Name: "inline"}, ytv1.ComponentStatus{State: ytv1.ComponentStateReady}))
		})
	}
}
