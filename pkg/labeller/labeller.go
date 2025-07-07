package labeller

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// Labeller defines component names, labels and addresses.
type Labeller struct {
	Namespace string

	// Name of YTsaurus cluster.
	ClusterName string

	// Name of resource which defines this component.
	ResourceName string

	ComponentType consts.ComponentType

	// An optional name identifying a group of instances of the type above.
	// Role for proxies, instance group name for nodes, may be empty.
	InstanceGroup string

	// K8s cluster domain, usually "cluster.local".
	ClusterDomain string

	Annotations map[string]string

	// Do not include resource name into component name.
	UseShortNames bool
}

func (l *Labeller) ForComponent(component consts.ComponentType, instanceGroup string) *Labeller {
	cl := *l
	cl.ComponentType = component
	cl.InstanceGroup = instanceGroup
	return &cl
}

func (l *Labeller) GetNamespace() string {
	return l.Namespace
}

func (l *Labeller) GetClusterName() string {
	return l.ClusterName
}

func (l *Labeller) GetClusterDomain() string {
	return l.ClusterDomain
}

// getGroupName converts <name> into <name>[-group]
func (l *Labeller) getGroupName(name string) string {
	if l.InstanceGroup != "" && l.InstanceGroup != consts.DefaultName {
		name += "-" + l.InstanceGroup
	}
	return name
}

// getName converts <name> into <name>[-group][-infix][-resource]
func (l *Labeller) getName(name, infix string) string {
	name = l.getGroupName(name)
	if infix != "" {
		name += "-" + infix
	}
	if !l.UseShortNames {
		// NOTE: It would be better add resource as prefix rather than as suffix ¯\_(ツ)_/¯.
		name += "-" + l.ResourceName
	}
	return name
}

// GetFullComponentName Returns CamelCase component type with instance group.
func (l *Labeller) GetFullComponentName() string {
	// NOTE: Class name is not CamelCase.
	return l.getGroupName(string(l.ComponentType))
}

func (l *Labeller) GetInstanceGroup() string {
	if l.InstanceGroup == consts.DefaultName {
		return ""
	}
	return l.InstanceGroup
}

func (l *Labeller) GetServerStatefulSetName() string {
	return l.getName(consts.ComponentStatefulSetPrefix(l.ComponentType), "")
}

func (l *Labeller) GetHeadlessServiceName() string {
	return l.getName(consts.ComponentServicePrefix(l.ComponentType), "")
}

func (l *Labeller) GetBalancerServiceName() string {
	// NOTE: For non-short names "-lb-" is inside ¯\_(ツ)_/¯.
	return l.getName(consts.ComponentServicePrefix(l.ComponentType), "lb")
}

func (l *Labeller) GetHeadlessServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		l.GetHeadlessServiceName(),
		l.GetNamespace(),
		l.ClusterDomain)
}

func (l *Labeller) GetBalancerServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		l.GetBalancerServiceName(),
		l.GetNamespace(),
		l.ClusterDomain)
}

func (l *Labeller) GetInstanceAddressWildcard() string {
	return fmt.Sprintf("*.%s.%s.svc.%s",
		l.GetHeadlessServiceName(),
		l.GetNamespace(),
		l.ClusterDomain)
}

func (l *Labeller) GetInstanceAddressPort(index, port int) string {
	// NOTE: https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/
	return fmt.Sprintf("%s-%d.%s.%s.svc.%s:%d",
		l.GetServerStatefulSetName(),
		index,
		l.GetHeadlessServiceName(),
		l.GetNamespace(),
		l.ClusterDomain,
		port)
}

// GetComponentLabel Returns lower case hyphenated component type without name part.
func (l *Labeller) GetComponentLabel() string {
	return consts.ComponentLabel(l.ComponentType)
}

// GetFullComponentLabel Returns lower case hyphenated component type with name part.
func (l *Labeller) GetFullComponentLabel() string {
	// NOTE: Resulting name does not include resource name, not so full ¯\_(ツ)_/¯.
	return l.getGroupName(l.GetComponentLabel())
}

func (l *Labeller) GetSecretName() string {
	return fmt.Sprintf("%s-secret", l.GetFullComponentLabel())
}

func (l *Labeller) GetMainConfigMapName() string {
	return fmt.Sprintf("%s-config", l.GetFullComponentLabel())
}

func (l *Labeller) GetSidecarConfigMapName(name string) string {
	return fmt.Sprintf("%s-%s-config", l.GetFullComponentLabel(), name)
}

func (l *Labeller) GetInitJobName(name string) string {
	return fmt.Sprintf("%s-init-job-%s", l.GetFullComponentLabel(), strings.ToLower(name))
}

func (l *Labeller) GetPodsRemovingStartedCondition() string {
	return fmt.Sprintf("%sPodsRemovingStarted", l.GetFullComponentName())
}

func (l *Labeller) GetPodsRemovedCondition() string {
	return fmt.Sprintf("%sPodsRemoved", l.GetFullComponentName())
}

func (l *Labeller) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   l.GetNamespace(),
		Labels:      l.GetMetaLabelMap(false),
		Annotations: l.Annotations,
	}
}

func (l *Labeller) GetInitJobObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        "ytsaurus-init",
		Namespace:   l.GetNamespace(),
		Labels:      l.GetMetaLabelMap(true),
		Annotations: l.Annotations,
	}
}

func (l *Labeller) GetInstanceLabelValue(isInitJob bool) string {
	// NOTE: Prefix is not cluster name as it was documented before ¯\_(ツ)_/¯.
	result := fmt.Sprintf("%s-%s", l.ResourceName, l.GetFullComponentLabel())
	if isInitJob {
		result = fmt.Sprintf("%s-%s", result, "init-job")
	}
	return result
}

func (l *Labeller) GetComponentTypeLabelValue(isInitJob bool) string {
	if isInitJob {
		return fmt.Sprintf("%s-%s", l.GetComponentLabel(), "init-job")
	}
	return l.GetComponentLabel()
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
		client.InNamespace(l.GetNamespace()),
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
		"app.kubernetes.io/component": l.GetFullComponentLabel(),
		// This is supposed to be the name of a higher level application
		// that this app is part of: yt-<cluster_name>.
		"app.kubernetes.io/part-of": l.GetPartOfLabelValue(),
		// Uppercase looks awful, even though it is more typical for k8s.
		"app.kubernetes.io/managed-by": "ytsaurus-k8s-operator",
		// It is nice to have the cluster name as a label.
		// Template: <cluster_name>.
		// NOTE: Previously this was "<resource_name>" by mistake ¯\_(ツ)_/¯.
		consts.YTClusterLabelName: l.GetClusterName(),
		// This label is used to check pods for readiness during updates.
		// The name isn't quite right, but we keep it for backwards compatibility.
		// Template: <resource_name>-yt-<component_type>-<instance_group>[-init-job].
		// NOTE: Prefix is not cluster name as it was documented before ¯\_(ツ)_/¯.
		consts.YTComponentLabelName: l.GetInstanceLabelValue(isInitJob),
	}
}

func (l *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := l.GetMetaLabelMap(false)

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}
