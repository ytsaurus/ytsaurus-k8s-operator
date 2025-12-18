package consts

import (
	"fmt"
)

const (
	YTClusterLabelName = "ytsaurus.tech/cluster-name"

	YTComponentLabelName = "yt_component"
	YTMetricsLabelName   = "yt_metrics"

	DescriptionAnnotation           = "kubernetes.io/description"
	ConfigOverridesVersionLabelName = "ytsaurus.tech/config-overrides-version"

	YTOperatorInstanceLabelName = "ytsaurus.tech/operator-instance"
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
	}

	panic(fmt.Sprintf("Unknown component type: %s", component))
}
