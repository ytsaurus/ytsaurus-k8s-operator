package consts

import (
	"fmt"
)

const (
	YTClusterLabelName = "ytsaurus.tech/cluster-name"

	YTComponentLabelName = "yt_component"
	YTMetricsLabelName   = "yt_metrics"

	DescriptionAnnotationName = "kubernetes.io/description"

	// Version of overrides observed at the time of generation.
	ConfigOverridesVersionAnnotationName = "ytsaurus.tech/config-overrides-version"

	// Config hash is computed from configmap data.
	ConfigHashAnnotationName = "ytsaurus.tech/config-hash"

	// Instance hash is computed from template of pod spec.
	InstanceHashAnnotationName = "ytsaurus.tech/instance-hash"

	// Observed generation of owner resource at the time of last update.
	ObservedGenerationAnnotationName = "ytsaurus.tech/observed-generation"

	YTOperatorInstanceLabelName = "ytsaurus.tech/operator-instance"
	ImageHeaterLabelName        = "ytsaurus.tech/image-heater"
)

func ComponentLabel(component ComponentType) string {
	// TODO(achulkov2): We should probably use `ytsaurus` instead of `yt` everywhere, but
	// it will be an inconvenient change that requires all statefulsets to be recreated.
	switch component {
	case MasterType:
		return "yt-master"
	case MasterCacheType:
		return "yt-master-cache"
	case CypressProxyType:
		return "yt-cypress-proxy"
	case DiscoveryType:
		return "yt-discovery"
	case SchedulerType:
		return "yt-scheduler"
	case ControllerAgentType:
		return "yt-controller-agent"
	case DataNodeType:
		return "yt-data-node"
	case ExecNodeType:
		return "yt-exec-node"
	case TabletNodeType:
		return "yt-tablet-node"
	case OffshoreDataGatewayType:
		return "yt-offshore-data-gateway"
	case HttpProxyType:
		return "yt-http-proxy"
	case RpcProxyType:
		return "yt-rpc-proxy"
	case TcpProxyType:
		return "yt-tcp-proxy"
	case KafkaProxyType:
		return "yt-kafka-proxy"
	case QueueAgentType:
		return "yt-queue-agent"
	case QueryTrackerType:
		return "yt-query-tracker"
	case YqlAgentType:
		return "yt-yql-agent"
	case StrawberryControllerType:
		return "yt-strawberry-controller"
	case BundleControllerType:
		return "yt-bundle-controller"
	case TabletBalancerType:
		return "yt-tablet-balancer"
	case ChytType:
		return "yt-chyt"
	case SpytType:
		return "yt-spyt"
	case YtsaurusClientType:
		return "yt-client"
	case UIType:
		return "yt-ui"
	case TimbertruckType:
		return "yt-timbertruck"
	case ImageHeaterType:
		return "yt-image-heater"
	}

	panic(fmt.Sprintf("Unknown component type: %s", component))
}
