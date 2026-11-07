package ytcluster

import (
	"testing"

	"github.com/stretchr/testify/require"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestGetInstanceCount(t *testing.T) {
	instance := ytv1.InstanceSpec{InstanceCount: 3}
	resource := &ytv1.Ytsaurus{Spec: ytv1.YtsaurusSpec{
		Discovery:            ytv1.DiscoverySpec{InstanceSpec: instance},
		PrimaryMasters:       ytv1.PrimaryMastersSpec{MastersSpec: ytv1.MastersSpec{InstanceSpec: instance}},
		SecondaryMasters:     []ytv1.SecondaryMastersSpec{{MastersSpec: ytv1.MastersSpec{InstanceSpec: instance, MasterConnectionSpec: ytv1.MasterConnectionSpec{CellTag: 2}}}},
		HTTPProxies:          []ytv1.HTTPProxiesSpec{{InstanceSpec: instance, Role: "default"}},
		RPCProxies:           []ytv1.RPCProxiesSpec{{InstanceSpec: instance, Role: "default"}},
		TCPProxies:           []ytv1.TCPProxiesSpec{{InstanceSpec: instance, Role: "default"}},
		KafkaProxies:         []ytv1.KafkaProxiesSpec{{InstanceSpec: instance, Role: "default"}},
		ExecNodes:            []ytv1.ExecNodesSpec{{InstanceSpec: instance, Name: "exec"}},
		DataNodes:            []ytv1.DataNodesSpec{{InstanceSpec: instance, Name: "data"}},
		TabletNodes:          []ytv1.TabletNodesSpec{{InstanceSpec: instance, Name: "tablet"}},
		MasterCaches:         &ytv1.MasterCachesSpec{InstanceSpec: instance},
		Schedulers:           &ytv1.SchedulersSpec{InstanceSpec: instance},
		ControllerAgents:     &ytv1.ControllerAgentsSpec{InstanceSpec: instance},
		StrawberryController: &ytv1.StrawberryControllerSpec{},
		QueryTrackers:        &ytv1.QueryTrackerSpec{InstanceSpec: instance},
		YQLAgents:            &ytv1.YQLAgentSpec{InstanceSpec: instance},
		QueueAgents:          &ytv1.QueueAgentSpec{InstanceSpec: instance},
		CypressProxies:       &ytv1.CypressProxiesSpec{InstanceSpec: instance},
		BundleController:     &ytv1.BundleControllerSpec{InstanceSpec: instance},
		TabletBalancer:       &ytv1.TabletBalancerSpec{InstanceSpec: instance},
		UI:                   &ytv1.UISpec{InstanceCount: 3},
	}}
	cluster := NewCluster(resource)
	for _, spec := range cluster.GetComponentSpecs() {
		component := spec.GetComponent()
		t.Run(component.String(), func(t *testing.T) {
			expected := 3
			if component.Type == ytv1.StrawberryControllerType {
				expected = 1
			}
			require.Equal(t, expected, cluster.GetInstanceCount(component))
		})
	}

	t.Run("additional specs", func(t *testing.T) {
		nodes := &ytv1.ExecNodesSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 5}, Name: "attached"}
		cluster.ExecNodes = append(cluster.ExecNodes, nodes)
		cluster.Components[nodes.GetComponent()] = nodes
		require.Equal(t, 5, cluster.GetInstanceCount(nodes.GetComponent()))
		nodes.InstanceCount = 7
		require.Equal(t, 7, cluster.GetInstanceCount(nodes.GetComponent()))
	})

	t.Run("unknown component", func(t *testing.T) {
		require.Zero(t, cluster.GetInstanceCount(ytv1.Component{Type: "unknown"}))
		for _, spec := range cluster.GetComponentSpecs() {
			component := spec.GetComponent()
			component.Name = "missing"
			require.Zero(t, cluster.GetInstanceCount(component))
		}
	})

	t.Run("omitted optional specs", func(t *testing.T) {
		empty := NewCluster(&ytv1.Ytsaurus{})
		for _, spec := range cluster.GetComponentSpecs() {
			component := spec.GetComponent()
			require.Zero(t, empty.GetInstanceCount(component))
		}
	})
}
