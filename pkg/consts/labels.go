package consts

import (
	"fmt"
)

const YTClusterLabelName = "ytsaurus.tech/cluster-name"
const YTComponentLabelName = "yt_component"
const YTMetricsLabelName = "yt_metrics"

func ComponentLabel(component ComponentType) string {
	// TODO(achulkov2): We should probably use `ytsaurus` instead of `yt` everywhere, but
	// it will be an inconvenient change that requires all statefulsets to be recreated.
	switch component {
	case MasterType:
		return "yt-master"
	case MasterCacheType:
		return "yt-master-cache"
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
	case ChytType:
		return "yt-chyt"
	case SpytType:
		return "yt-spyt"
	case YtsaurusClientType:
		return "yt-client"
	case UIType:
		return "yt-ui"
	}

	panic(fmt.Sprintf("Unknown component type: %s", component))
}
