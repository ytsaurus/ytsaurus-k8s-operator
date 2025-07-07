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
	case TcpProxyType:
		return "tcp-proxies"
	case KafkaProxyType:
		return "kafka-proxies"
	case YqlAgentType:
		return "yql-agents"
	case TimbertruckType: 
		return "timbertrucks"
	}

	panic(fmt.Sprintf("No service is defined for component type: %s", component))
}

func GetServiceKebabCase(component ComponentType) string {
	switch component {
	case ControllerAgentType:
		return "controller-agent"
	case DataNodeType:
		return "data-node"
	case DiscoveryType:
		return "discovery"
	case ExecNodeType:
		return "exec-node"
	case HttpProxyType:
		return "http-proxy"
	case MasterCacheType:
		return "master-cache"
	case MasterType:
		return "master"
	case QueryTrackerType:
		return "query-tracker"
	case QueueAgentType:
		return "queue-agent"
	case RpcProxyType:
		return "rpc-proxy"
	case SchedulerType:
		return "scheduler"
	case StrawberryControllerType:
		return "strawberry-controller"
	case TabletNodeType:
		return "tablet-node"
	case TcpProxyType:
		return "tcp-proxy"
	case KafkaProxyType:
		return "kafka-proxy"
	case YqlAgentType:
		return "yql-agent"
	case TimbertruckType: 
		return "timbertruck"
	}
	panic(fmt.Sprintf("No kebab case service name is defined for component type: %s", component))
}

func GetStatefulSetPrefix(component ComponentType) string {
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
	case TcpProxyType:
		return "tp"
	case KafkaProxyType:
		return "kp"
	case YqlAgentType:
		return "yqla"
	case TimbertruckType: 
		return "tt"
	}
	return ""
}

func GetMicroservicePrefix(component ComponentType) string {
	switch component {
	case StrawberryControllerType:
		return "strawberry"
	case UIType:
		return "ui"
	}
	return ""
}

func GetShortName(component ComponentType) string {
	stsPrefix := GetStatefulSetPrefix(component)
	if stsPrefix != "" {
		return stsPrefix
	}
	return GetMicroservicePrefix(component)
}

func ComponentStatefulSetPrefix(component ComponentType) string {
	shortTypeName := GetStatefulSetPrefix(component)
	if shortTypeName != "" {
		return shortTypeName
	}
	panic(fmt.Sprintf("No stateful set is defined for component type: %s", component))
}
