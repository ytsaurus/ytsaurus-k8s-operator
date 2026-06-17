package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// allClusterStates lists every possible ClusterState value so we can emit a
// series for each state (value 1 = active, 0 = inactive) as an enum metric.
var allClusterStates = []ytv1.ClusterState{
	ytv1.ClusterStateCreated,
	ytv1.ClusterStateInitializing,
	ytv1.ClusterStatePreparing,
	ytv1.ClusterStateRunning,
	ytv1.ClusterStateReconfiguration,
	ytv1.ClusterStateMaintenance,
	ytv1.ClusterStateUpdating,
	ytv1.ClusterStateUpdateCanceled,
	ytv1.ClusterStateUpdateBlocked,
	ytv1.ClusterStateUpdateFinished,
}

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var clusterStateGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "ytop",
		Subsystem: "cluster",
		Name:      "state",
		Help:      "Current state of a Ytsaurus cluster. For each (cluster, cluster_namespace, state) tuple the value is 1 when that state is active, 0 otherwise.",
	},
	[]string{"cluster", "cluster_namespace", "state"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(clusterStateGauge)
}

// ObserveClusterState records the current cluster state as a Prometheus metric.
// It sets the active state's series to 1 and all other known states to 0.
func ObserveClusterState(cluster, clusterNamespace string, currentState ytv1.ClusterState) {
	for _, s := range allClusterStates {
		value := 0.0
		if s == currentState {
			value = 1.0
		}
		clusterStateGauge.WithLabelValues(cluster, clusterNamespace, string(s)).Set(value)
	}
}
