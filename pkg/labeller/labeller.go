package labeller

import (
	"fmt"
	"strings"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
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
	ComponentType  string
	ComponentLabel string
	ComponentName  string
	Annotations    map[string]string
}

func (l *Labeller) GetClusterName() string {
	return l.ObjectMeta.Name
}

func (l *Labeller) GetComponentType() string {
	if l.ComponentType != "" {
		return l.ComponentType
	}
	return l.ComponentLabel
}

func (l *Labeller) GetSecretName() string {
	return fmt.Sprintf("%s-secret", l.ComponentLabel)
}

func (l *Labeller) GetMainConfigMapName() string {
	return fmt.Sprintf("%s-config", l.ComponentLabel)
}

func (l *Labeller) GetSidecarConfigMapName(name string) string {
	return fmt.Sprintf("%s-%s-config", l.ComponentLabel, name)
}

func (l *Labeller) GetInitJobName(name string) string {
	return fmt.Sprintf("%s-init-job-%s", l.ComponentLabel, strings.ToLower(name))
}

func (l *Labeller) GetPodsRemovingStartedCondition() string {
	return fmt.Sprintf("%sPodsRemovingStarted", l.ComponentName)
}

func (l *Labeller) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   l.ObjectMeta.Namespace,
		Labels:      l.GetMetaLabelMap(false),
		Annotations: l.Annotations,
	}
}

func (l *Labeller) GetInitJobObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        "ytsaurus-init",
		Namespace:   l.ObjectMeta.Namespace,
		Labels:      l.GetMetaLabelMap(true),
		Annotations: l.Annotations,
	}
}

func (l *Labeller) GetInstanceLabelValue(isInitJob bool) string {
	result := fmt.Sprintf("%s-%s", l.GetClusterName(), l.ComponentLabel)
	if isInitJob {
		result = fmt.Sprintf("%s-%s", result, "init-job")
	}
	return result
}

func (l *Labeller) GetComponentTypeLabelValue(isInitJob bool) string {
	if isInitJob {
		return fmt.Sprintf("%s-%s", l.GetComponentType(), "init-job")
	}
	return l.GetComponentType()
}

func (l *Labeller) GetPartOfLabelValue() string {
	// TODO(achulkov2): Change this from `yt` to `ytsaurus` at the same time as all other label values.
	return fmt.Sprintf("yt-%s", l.GetClusterName())
}

func (l *Labeller) GetSelectorLabelMap() map[string]string {
	return map[string]string{
		consts.YTComponentLabelName: l.GetInstanceLabelValue(false),
	}
}

func (l *Labeller) GetListOptions() []client.ListOption {
	return []client.ListOption{
		client.InNamespace(l.ObjectMeta.Namespace),
		client.MatchingLabels(l.GetSelectorLabelMap()),
	}
}

func (l *Labeller) GetMetaLabelMap(isInitJob bool) map[string]string {
	return map[string]string{
		// This is supposed to be the name of the application. It makes
		// sense to separate init jobs from the main components. It does
		// not contain the name of the instance group for easier monitoring
		// configuration.
		// Template: yt-<component_type>[-init-job].
		"app.kubernetes.io/name": l.GetComponentTypeLabelValue(isInitJob),
		// Template: yt-<component_type>-<instance_group>.
		"app.kubernetes.io/component": l.ComponentLabel,
		// This is supposed to be the name of a higher level application
		// that this app is part of: yt-<cluster_name>.
		"app.kubernetes.io/part-of": l.GetPartOfLabelValue(),
		// Uppercase looks awful, even though it is more typical for k8s.
		"app.kubernetes.io/managed-by": "ytsaurus-k8s-operator",
		// It is nice to have the cluster name as a label.
		// Template: <cluster_name>.
		consts.YTClusterLabelName: l.GetClusterName(),
		// This label is used to check pods for readiness during updates.
		// The name isn't quite right, but we keep it for backwards compatibility.
		// Template: <cluster_name>-yt-<component_type>-<instance_group>[-init-job].
		consts.YTComponentLabelName: l.GetInstanceLabelValue(isInitJob),
	}
}

func (l *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := l.GetMetaLabelMap(false)

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}

func GetPodsRemovedCondition(componentName string) string {
	return fmt.Sprintf("%sPodsRemoved", componentName)
}
