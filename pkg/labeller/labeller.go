package labeller

import (
	"fmt"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FetchableObject struct {
	Name   string
	Object client.Object
}

type Labeller struct {
	ObjectMeta                 *metav1.ObjectMeta
	ComponentObjectsNamePrefix string
	ComponentFullName          string

	extraLabels map[string]string
	annotations map[string]string

	instanceName string

	objectsNamePrefixBase string
}

func NewLabellerForGlobalComponent(objectMeta *metav1.ObjectMeta, name consts.ComponentType, objectsNamePrefix string, extraLabels, annotations map[string]string) Labeller {
	if extraLabels == nil {
		extraLabels = make(map[string]string)
	}

	l := Labeller{
		ObjectMeta: objectMeta,

		ComponentFullName:          string(name),
		ComponentObjectsNamePrefix: objectsNamePrefix,

		objectsNamePrefixBase: objectsNamePrefix,

		extraLabels: extraLabels,
		annotations: annotations,
	}

	if len(l.extraLabels) > 0 {
		for k := range l.getDefaultLabelsMap(false) {
			delete(l.extraLabels, k)
		}
	}

	return l
}

func NewLabellerForComponentInstance(objectMeta *metav1.ObjectMeta, nameBase consts.ComponentType, objectsNamePrefixBase, instanceName string, extraLabels, annotations map[string]string) Labeller {
	l := NewLabellerForGlobalComponent(objectMeta, nameBase, objectsNamePrefixBase, extraLabels, annotations)
	l.instanceName = instanceName

	if instanceName != "" && instanceName != consts.DefaultName {
		l.ComponentFullName = fmt.Sprintf("%s-%s", nameBase, instanceName)
		l.ComponentObjectsNamePrefix = fmt.Sprintf("%s-%s", objectsNamePrefixBase, instanceName)
	}

	return l
}

func (l *Labeller) GetClusterName() string {
	return l.ObjectMeta.Name
}

func (l *Labeller) GetSecretName() string {
	return fmt.Sprintf("%s-secret", l.ComponentObjectsNamePrefix)
}

func (l *Labeller) GetMainConfigMapName() string {
	return fmt.Sprintf("%s-config", l.ComponentObjectsNamePrefix)
}

func (l *Labeller) GetSidecarConfigMapName(name string) string {
	return fmt.Sprintf("%s-%s-config", l.ComponentObjectsNamePrefix, name)
}

func (l *Labeller) GetInitJobName(name string) string {
	return fmt.Sprintf("%s-init-job-%s", l.ComponentObjectsNamePrefix, strings.ToLower(name))
}

func (l *Labeller) GetPodsRemovingStartedCondition() string {
	return fmt.Sprintf("%sPodsRemovingStarted", l.ComponentFullName)
}

func (l *Labeller) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   l.ObjectMeta.Namespace,
		Labels:      l.GetMetaLabelMap(false),
		Annotations: l.annotations,
	}
}

func (l *Labeller) GetInitJobObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        "ytsaurus-init",
		Namespace:   l.ObjectMeta.Namespace,
		Labels:      l.GetMetaLabelMap(true),
		Annotations: l.annotations,
	}
}

func (l *Labeller) GetYTLabelValue(isInitJob bool) string {
	result := fmt.Sprintf("%s-%s", l.ObjectMeta.Name, l.ComponentObjectsNamePrefix)
	if isInitJob {
		result = fmt.Sprintf("%s-%s", result, "init-job")
	}
	return result
}

func (l *Labeller) GetSelectorLabelMap() map[string]string {
	return map[string]string{
		consts.YTComponentLabelName: l.GetYTLabelValue(false),
	}
}

func (l *Labeller) GetListOptions() []client.ListOption {
	return []client.ListOption{
		client.InNamespace(l.ObjectMeta.Namespace),
		client.MatchingLabels(l.GetSelectorLabelMap()),
	}
}

func (l *Labeller) getDefaultLabelsMap(isInitJob bool) map[string]string {
	instanceName := l.objectsNamePrefixBase
	if l.instanceName != "" {
		instanceName = l.instanceName
	}

	return map[string]string{
		"app.kubernetes.io/name":       "Ytsaurus",
		"app.kubernetes.io/component":  l.objectsNamePrefixBase,
		"app.kubernetes.io/instance":   instanceName,
		"app.kubernetes.io/part-of":    l.ObjectMeta.Name,
		"app.kubernetes.io/managed-by": "Ytsaurus-k8s-operator",
		consts.YTComponentLabelName:    l.GetYTLabelValue(isInitJob),
	}
}

func (l *Labeller) GetMetaLabelMap(isInitJob bool) map[string]string {
	labels := l.extraLabels
	for k, v := range l.getDefaultLabelsMap(isInitJob) {
		labels[k] = v
	}

	return labels
}

func (l *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := l.GetMetaLabelMap(false)

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}

func GetPodsRemovedCondition(componentName string) string {
	return fmt.Sprintf("%sPodsRemoved", componentName)
}
