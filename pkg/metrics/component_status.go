package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// allSyncStatuses lists every possible component SyncStatus string value.
// These must stay in sync with the SyncStatus constants in pkg/components/component.go.
// Strings are used here instead of the SyncStatus type to avoid a circular import
// (pkg/components already imports pkg/metrics).
//
//nolint:gochecknoglobals // constants for metric label enumeration
var allSyncStatuses = []string{
	"",           // SyncStatusUndefined
	"Ready",      // SyncStatusReady
	"Started",    // SyncStatusStarted
	"Blocked",    // SyncStatusBlocked
	"Pending",    // SyncStatusPending
	"NeedUpdate", // SyncStatusNeedUpdate
	"Updating",   // SyncStatusUpdating
}

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var componentSyncStatus = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "ytop",
		Subsystem: "component",
		Name:      "sync_status",
		Help: "Sync status of a Ytsaurus component. " +
			"For each (cluster, cluster_namespace, component_name, sync_status) tuple " +
			"the value is 1 when that status is active, 0 otherwise.",
	},
	[]string{"cluster", "cluster_namespace", "component_name", "sync_status"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(componentSyncStatus)
}

// ObserveComponentSyncStatus records the current sync status for a single component.
// activeSyncStatus must be the string value of a components.SyncStatus constant.
// It sets the active status's series to 1 and all other known statuses to 0.
func ObserveComponentSyncStatus(cluster, clusterNamespace, componentName, activeSyncStatus string) {
	for _, s := range allSyncStatuses {
		value := 0.0
		if s == activeSyncStatus {
			value = 1.0
		}
		componentSyncStatus.WithLabelValues(cluster, clusterNamespace, componentName, s).Set(value)
	}
}
