package consts

import (
	"fmt"
)

func ComponentServicePrefix(component ComponentType) string {
	switch component {
	case ControllerAgentType:
		return "controller-agents"
	case DataNodeType:
		return "data-nodes"
	case DiscoveryType:
		return "discovery"
	case ExecNodeType:
		return "exec-nodes"
	case HttpProxyType:
		return "http-proxies"
	case MasterCacheType:
		return "master-caches"
	case MasterType:
		return "masters"
	case QueryTrackerType:
		return "query-trackers"
	case QueueAgentType:
		return "queue-agents"
	case RpcProxyType:
		return "rpc-proxies"
	case SchedulerType:
		return "schedulers"
	case StrawberryControllerType:
		return "strawberry"
	case TabletNodeType:
		return "tablet-nodes"
	case TabletBalancerType:
		return "tablet-balancer"
	case TcpProxyType:
		return "tcp-proxies"
	case KafkaProxyType:
		return "kafka-proxies"
	case YqlAgentType:
		return "yql-agents"
	}

	panic(fmt.Sprintf("No service is defined for component type: %s", component))
}

func ComponentStatefulSetPrefix(component ComponentType) string {
	switch component {
	case ControllerAgentType:
		return "ca"
	case DataNodeType:
		return "dnd"
	case DiscoveryType:
		return "ds"
	case ExecNodeType:
		return "end"
	case HttpProxyType:
		return "hp"
	case MasterCacheType:
		return "msc"
	case MasterType:
		return "ms"
	case QueryTrackerType:
		return "qt"
	case QueueAgentType:
		return "qa"
	case RpcProxyType:
		return "rp"
	case SchedulerType:
		return "sch"
	case TabletNodeType:
		return "tnd"
	case TabletBalancerType:
		return "tb"
	case TcpProxyType:
		return "tp"
	case KafkaProxyType:
		return "kp"
	case YqlAgentType:
		return "yqla"
	}

	panic(fmt.Sprintf("No stateful set is defined for component type: %s", component))
}
