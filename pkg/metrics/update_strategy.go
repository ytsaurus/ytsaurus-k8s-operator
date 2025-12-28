package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

//nolint:gochecknoglobals // Prometheus metrics are package-level for registration and reuse.
var onDeleteWaitSeconds = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "ytop",
		Subsystem: "update_strategy",
		Name:      "on_delete_wait_seconds",
		Help:      "Seconds a component has been waiting in OnDelete mode since the mode started.",
	},
	[]string{"cluster", "component"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(onDeleteWaitSeconds)
}

func ObserveOnDeleteWait(cluster, component string, startedAt *metav1.Time) {
	lbl := []string{cluster, component}
	if startedAt == nil || startedAt.IsZero() {
		onDeleteWaitSeconds.DeleteLabelValues(lbl...)
		return
	}
	onDeleteWaitSeconds.WithLabelValues(lbl...).Set(time.Since(startedAt.Time).Seconds())
}
