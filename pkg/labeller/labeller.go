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
	Annotations                map[string]string

	instanceName          string
	nameBase              consts.ComponentType
	objectsNamePrefixBase string
}

func NewLabeller(objectMeta *metav1.ObjectMeta, nameBase consts.ComponentType, objectsNamePrefixBase string) *Labeller {
	l := &Labeller{
		ObjectMeta: objectMeta,

		nameBase:              nameBase,
		objectsNamePrefixBase: objectsNamePrefixBase,
	}

	return l
}

func (l *Labeller) WithComponentInstanceName(instanceName string) *Labeller {
	l.instanceName = instanceName
	return l
}

func (l *Labeller) WithAnnotations(annotations map[string]string) *Labeller {
	l.Annotations = annotations
	return l
}

func (l *Labeller) Build() Labeller {
	l.ComponentFullName = string(l.nameBase)
	l.ComponentObjectsNamePrefix = l.objectsNamePrefixBase

	if l.instanceName != "" && l.instanceName != consts.DefaultName {
		l.ComponentFullName = fmt.Sprintf("%s-%s", l.nameBase, l.instanceName)
		l.ComponentObjectsNamePrefix = fmt.Sprintf("%s-%s", l.objectsNamePrefixBase, l.instanceName)
	}

	return *l
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
	return fmt.Sprintf("%s-%s-config", l.ComponentLabel, name)
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

func (l *Labeller) GetMetaLabelMap(isInitJob bool) map[string]string {
	instanceName := l.objectsNamePrefixBase
	if l.instanceName != "" {
		instanceName = l.objectsNamePrefixBase
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

func (l *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := l.GetMetaLabelMap(false)

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}

func GetPodsRemovedCondition(componentName string) string {
	return fmt.Sprintf("%sPodsRemoved", componentName)
}
