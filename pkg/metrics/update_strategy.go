package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var strategyOnDeleteWatingTimeSeconds = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "ytop",
		Subsystem: "strategy_on_delete",
		Name:      "waiting_time_seconds",
		Help:      "Seconds a component has been waiting in OnDelete mode since the mode started.",
	},
	[]string{"cluster", "cluster_namespace", "component_name"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(strategyOnDeleteWatingTimeSeconds)
}

func ObserveOnDeleteWait(cluster, cluster_namespace, component_name string, startedAt *metav1.Time) {
	lbl := []string{cluster, cluster_namespace, component_name}
	if startedAt == nil || startedAt.IsZero() {
		strategyOnDeleteWatingTimeSeconds.DeleteLabelValues(lbl...)
		return
	}
	strategyOnDeleteWatingTimeSeconds.WithLabelValues(lbl...).Set(time.Since(startedAt.Time).Seconds())
}
