package consts

func ComponentCypressPath(component ComponentType) string {
	switch component {
	case ControllerAgentType:
		return "//sys/controller_agents/instances"
	case DataNodeType:
		return "//sys/data_nodes"
	case DiscoveryType:
		return "//sys/discovery_servers"
	case ExecNodeType:
		return "//sys/exec_nodes"
	case HttpProxyType:
		return "//sys/http_proxies"
	case MasterCacheType:
		return "//sys/master_caches"
	case MasterType:
		return "//sys/primary_masters"
	case QueryTrackerType:
		return "//sys/query_tracker/instances"
	case QueueAgentType:
		return "//sys/queue_agents/instances"
	case RpcProxyType:
		return "//sys/rpc_proxies"
	case SchedulerType:
		return "//sys/scheduler/instances"
	case TabletNodeType:
		return "//sys/tablet_nodes"
	case TcpProxyType:
		return "//sys/tcp_proxies/instances"
	case KafkaProxyType:
		return "//sys/kafka_proxies/instances"
	case YqlAgentType:
		return "//sys/yql_agent/instances"
	}
	return ""
}

const (
	LostVitalChunksCountPath      = "//sys/lost_vital_chunks/@count"
	ParityMissingChunksCountPath  = "//sys/parity_missing_chunks/@count"
	DataMissingChunksCountPath    = "//sys/data_missing_chunks/@count"
	UnsafelyPlacedChunksCountPath = "//sys/unsafely_placed_chunks/@count"
	QuorumMissingChunksCountPath  = "//sys/quorum_missing_chunks/@count"
)
