package labeller

import (
	"fmt"
	"strings"

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
	APIProxy       apiproxy.APIProxy
	ObjectMeta     *metav1.ObjectMeta
	ComponentLabel string
	ComponentName  string
	MonitoringPort int32
}

func (l *Labeller) GetClusterName() string {
	return l.ObjectMeta.Name
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
	return fmt.Sprintf("%sPodsRemovingStarted", l.ComponentName)
}

func (l *Labeller) GetPodsRemovedCondition() string {
	return fmt.Sprintf("%sPodsRemoved", l.ComponentName)
}

func (l *Labeller) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: l.ObjectMeta.Namespace,
		Labels:    l.GetMetaLabelMap(&name),
	}
}

func (l *Labeller) GetYTLabelValue() string {
	return fmt.Sprintf("%s-%s", l.ObjectMeta.Name, l.ComponentLabel)
}

func (l *Labeller) GetSelectorLabelMap(name *string) map[string]string {
	labels := map[string]string{
		consts.YTComponentLabelName: l.GetYTLabelValue(),
	}
	if name != nil {
		labels[consts.YTObjectName] = *name
	}

	return labels
}

func (l *Labeller) GetListOptions(name *string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(l.ObjectMeta.Namespace),
		client.MatchingLabels(l.GetSelectorLabelMap(name)),
	}
}

func (l *Labeller) GetMetaLabelMap(name *string) map[string]string {
	labels := map[string]string{
		"app.kubernetes.io/name":       "Ytsaurus",
		"app.kubernetes.io/instance":   l.ObjectMeta.Name,
		"app.kubernetes.io/component":  l.ComponentLabel,
		"app.kubernetes.io/managed-by": "Ytsaurus-k8s-operator",
		consts.YTComponentLabelName:    l.GetYTLabelValue(),
	}
	if name != nil {
		labels[consts.YTObjectName] = *name
	}
	return labels
}

func (l *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := l.GetMetaLabelMap(nil)

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}
