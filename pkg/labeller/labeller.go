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

	// Do not append resource name to names of (some) managed resources.
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

// appendGroupName converts <prefix> into <prefix>[-group]
func (l *Labeller) appendGroupName(name string) string {
	if l.InstanceGroup != "" && l.InstanceGroup != consts.DefaultName {
		name += "-" + l.InstanceGroup
	}
	return name
}

// managedResourceName returns <prefix><name>[-group]<suffix>
// NOTE: In some cases we always use "short" name regardless of UseShortNames ¯\_(ツ)_/¯.
func (l *Labeller) managedResourceShortName(prefix, name, suffix string) string {
	return prefix + l.appendGroupName(name) + suffix
}

// managedResourceLongName returns <prefix><name>[-group]<suffix>[-resource]
// NOTE: It would be better add control resource as prefix rather than as suffix ¯\_(ツ)_/¯.
// FIXME: Deprecate and remove long names - we don't want more than one cluster in namespace.
func (l *Labeller) managedResourceLongName(prefix, name, suffix string) string {
	result := l.managedResourceShortName(prefix, name, suffix)
	if !l.UseShortNames {
		result += "-" + l.ResourceName
	}
	return result
}

// GetComponentName Returns "<ComponentType>[-<InstanceGroup>]".
// NOTE: instance group comes from spec and can be non-CamelCase ¯\_(ツ)_/¯.
func (l *Labeller) GetComponentName() consts.ComponentName {
	return consts.ComponentName(l.appendGroupName(string(l.ComponentType)))
}

func (l *Labeller) GetInstanceGroup() string {
	if l.InstanceGroup == consts.DefaultName {
		return ""
	}
	return l.InstanceGroup
}

func (l *Labeller) GetServerStatefulSetName() string {
	return l.managedResourceLongName("", l.ComponentType.ShortName(), "")
}

func (l *Labeller) GetHeadlessServiceName() string {
	return l.managedResourceLongName("", l.ComponentType.PluralName(), "")
}

func (l *Labeller) GetBalancerServiceName() string {
	// NOTE: For long names "-lb-" is inside ¯\_(ツ)_/¯.
	return l.managedResourceLongName("", l.ComponentType.PluralName(), "-lb")
}

func (l *Labeller) GetMonitoringServiceName() string {
	// NOTE: Should be plural name but we messed up ¯\_(ツ)_/¯.
	return l.managedResourceShortName("yt-", l.ComponentType.SingularName(), "-monitoring")
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

func (l *Labeller) GetSecretName() string {
	return l.managedResourceShortName("yt-", l.ComponentType.SingularName(), "-secret")
}

func (l *Labeller) GetMainConfigMapName() string {
	return l.managedResourceShortName("yt-", l.ComponentType.SingularName(), "-config")
}

func (l *Labeller) GetSidecarConfigMapName(sidecarName string) string {
	return l.managedResourceShortName("yt-", l.ComponentType.SingularName(), "-"+sidecarName+"-config")
}

func (l *Labeller) GetInitJobName(initJobName string) string {
	return l.managedResourceShortName("yt-", l.ComponentType.SingularName(), "-init-job-"+strings.ToLower(initJobName))
}

func (l *Labeller) GetInitJobConfigMapName(initJobName string) string {
	// FIXME: Fix this madness.
	return l.managedResourceShortName(strings.ToLower(initJobName)+"-yt-", l.ComponentType.SingularName(), "-init-job-config")
}

func (l *Labeller) GetInitJobCompletedCondition(name string) string {
	return fmt.Sprintf("%s%vInitJobCompleted", name, l.GetComponentName())
}

func (l *Labeller) GetPodsRemovingStartedCondition() string {
	return fmt.Sprintf("%vPodsRemovingStarted", l.GetComponentName())
}

func (l *Labeller) GetPodsRemovedCondition() string {
	return fmt.Sprintf("%vPodsRemoved", l.GetComponentName())
}

func (l *Labeller) GetReadyCondition() string {
	return fmt.Sprintf("%vReady", l.GetComponentName())
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

// GetAppComponentLabelValue Returns lower case hyphenated component type with name part: yt-<singular>[-<group>]
func (l *Labeller) GetAppComponentLabelValue() string {
	// NOTE: Resulting name does not include resource name ¯\_(ツ)_/¯.
	return l.appendGroupName("yt-" + l.ComponentType.SingularName())
}

func (l *Labeller) GetYTComponentLabelValue(isInitJob bool) string {
	// NOTE: Prefix is not cluster name as it was documented before ¯\_(ツ)_/¯.
	result := l.ResourceName + "-" + l.GetAppComponentLabelValue()
	if isInitJob {
		result += "-init-job"
	}
	return result
}

func (l *Labeller) GetAppNameLabelValue(isInitJob bool) string {
	result := "yt-" + l.ComponentType.SingularName()
	if isInitJob {
		result += "-init-job"
	}
	return result
}

func (l *Labeller) GetAppPartOfLabelValue() string {
	// TODO(achulkov2): Change this from `yt` to `ytsaurus` at the same time as all other label values.
	return "yt-" + l.GetClusterName()
}

func (l *Labeller) GetSelectorLabelMap() map[string]string {
	return map[string]string{
		consts.YTComponentLabelName: l.GetYTComponentLabelValue(false),
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
		"app.kubernetes.io/name": l.GetAppNameLabelValue(isInitJob),
		// Template: yt-<component_type>-<instance_group>.
		"app.kubernetes.io/component": l.GetAppComponentLabelValue(),
		// This is supposed to be the name of a higher level application
		// that this app is part of: yt-<cluster_name>.
		"app.kubernetes.io/part-of": l.GetAppPartOfLabelValue(),
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
		consts.YTComponentLabelName: l.GetYTComponentLabelValue(isInitJob),
	}
}

func (l *Labeller) GetMonitoringMetaLabelMap() map[string]string {
	labels := l.GetMetaLabelMap(false)

	labels[consts.YTMetricsLabelName] = "true"

	return labels
}
