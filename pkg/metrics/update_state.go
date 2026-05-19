package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// allUpdateStates lists every possible UpdateState value so we can emit an enum-style metric.
//
//nolint:gochecknoglobals // constants for metric label enumeration
var allUpdateStates = []ytv1.UpdateState{
	ytv1.UpdateStateUndefined,
	ytv1.UpdateStateNone,
	ytv1.UpdateStateWaitingForImageHeater,
	ytv1.UpdateStatePossibilityCheck,
	ytv1.UpdateStateImpossibleToStart,
	ytv1.UpdateStateWaitingForSafeModeEnabled,
	ytv1.UpdateStateWaitingForTabletCellsSaving,
	ytv1.UpdateStateWaitingForTabletCellsRemovingStart,
	ytv1.UpdateStateWaitingForTabletCellsRemoved,
	ytv1.UpdateStateWaitingForImaginaryChunksAbsence,
	ytv1.UpdateStateWaitingForSnapshots,
	ytv1.UpdateStateWaitingForPodsRemoval,
	ytv1.UpdateStateWaitingForPodsCreation,
	ytv1.UpdateStateWaitingForMasterReady,
	ytv1.UpdateStateWaitingForMasterExitReadOnly,
	ytv1.UpdateStateWaitingForCypressPatch,
	ytv1.UpdateStateWaitingForTabletCellsRecovery,
	ytv1.UpdateStateWaitingForOpArchiveUpdate,
	ytv1.UpdateStateWaitingForSidecarsInitialize,
	ytv1.UpdateStateWaitingForQTStateUpdatingPrepare,
	ytv1.UpdateStateWaitingForQTStateUpdate,
	ytv1.UpdateStateWaitingForQAStateUpdatingPrepare,
	ytv1.UpdateStateWaitingForQAStateUpdate,
	ytv1.UpdateStateWaitingForYqlaUpdate,
	ytv1.UpdateStateWaitingForSafeModeDisabled,
	ytv1.UpdateStateWaitingForTimbertruckPrepared,
}

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var updateStateGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "ytop",
		Subsystem: "cluster",
		Name:      "update_state",
		Help: "Current update state of a Ytsaurus cluster. " +
			"For each (cluster, cluster_namespace, update_state) tuple " +
			"the value is 1 when that update state is active, 0 otherwise. " +
			"Only meaningful when ytop_cluster_state{state=\"Updating\"} == 1.",
	},
	[]string{"cluster", "cluster_namespace", "update_state"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(updateStateGauge)
}

// ObserveUpdateState records the current update state as a Prometheus metric.
// It sets the active state's series to 1 and all other known states to 0.
func ObserveUpdateState(cluster, clusterNamespace string, currentState ytv1.UpdateState) {
	for _, s := range allUpdateStates {
		value := 0.0
		if s == currentState {
			value = 1.0
		}
		updateStateGauge.WithLabelValues(cluster, clusterNamespace, string(s)).Set(value)
	}
}
