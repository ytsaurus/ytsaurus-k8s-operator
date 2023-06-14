package labeller

import (
	"fmt"
	"strings"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FetchableObject struct {
	Name   string
	Object client.Object
}

type Labeller struct {
	APIProxy       *apiproxy.APIProxy
	Ytsaurus       *ytv1.Ytsaurus
	ComponentLabel string
	ComponentName  string
	MonitoringPort int32
}

func (l *Labeller) GetClusterName() string {
	return l.Ytsaurus.Name
}

func (l *Labeller) GetSecretName() string {
	return fmt.Sprintf("%s-secret", l.ComponentLabel)
}

func (l *Labeller) GetMainConfigMapName() string {
	return fmt.Sprintf("%s-config", l.ComponentLabel)
}

func (l *Labeller) GetInitJobName(name string) string {
	return fmt.Sprintf("%s-init-job-%s", l.ComponentLabel, strings.ToLower(name))
}

func (l *Labeller) GetPodsRemovingStartedCondition() string {
	return fmt.Sprintf("%sPodsRemovingStarted", l.ComponentLabel)
}

func (l *Labeller) GetPodsRemovedCondition() string {
	return fmt.Sprintf("%sPodsRemoved", l.ComponentLabel)
}

func (l *Labeller) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: l.Ytsaurus.Namespace,
		Labels:    l.GetMetaLabelMap(),
	}
}

func (l *Labeller) GetYTLabelValue() string {
	return fmt.Sprintf("%s-%s", l.Ytsaurus.Name, l.ComponentLabel)
}

func (l *Labeller) GetSelectorLabelMap() map[string]string {
	return map[string]string{
		consts.YTComponentLabelName: l.GetYTLabelValue(),
	}
}

func (l *Labeller) GetListOptions() []client.ListOption {
	return []client.ListOption{
		client.InNamespace(l.Ytsaurus.Namespace),
		client.MatchingLabels(l.GetSelectorLabelMap()),
	}
}

func (l *Labeller) GetMetaLabelMap() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "Ytsaurus",
		"app.kubernetes.io/instance":   l.Ytsaurus.Name,
		"app.kubernetes.io/component":  l.ComponentLabel,
		"app.kubernetes.io/managed-by": "Ytsaurus-k8s-operator",
		consts.YTComponentLabelName:    l.GetYTLabelValue(),
	}
}

func (r *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := r.GetMetaLabelMap()

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}
