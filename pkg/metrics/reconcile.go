package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var reconcileTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "ytop",
		Subsystem: "reconcile",
		Name:      "total",
		Help:      "Total number of Ytsaurus reconcile attempts, partitioned by result (success or error).",
	},
	[]string{"cluster", "cluster_namespace", "result"},
)

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var reconcileDurationSeconds = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "ytop",
		Subsystem: "reconcile",
		Name:      "duration_seconds",
		Help:      "Duration of a single Ytsaurus reconcile cycle in seconds.",
		Buckets:   prometheus.DefBuckets,
	},
	[]string{"cluster", "cluster_namespace"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(reconcileTotal)
	ctrlmetrics.Registry.MustRegister(reconcileDurationSeconds)
}

// ObserveReconcile records one completed reconcile cycle.
// Call it with the cluster name, its namespace, the wall-clock duration, and any error returned.
func ObserveReconcile(cluster, clusterNamespace string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	reconcileTotal.WithLabelValues(cluster, clusterNamespace, result).Inc()
	reconcileDurationSeconds.WithLabelValues(cluster, clusterNamespace).Observe(duration.Seconds())
}
