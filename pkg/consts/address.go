package consts

const (
	HTTPPortName                  = "http"
	HTTPSPortName                 = "https"
	YTRPCPortName                 = "rpc"
	YTPublicRPCPortName           = "public-rpc"
	KafkaPortName                 = "kafka"
	YTMonitoringContainerPortName = "metrics"
	YTMonitoringServicePortName   = "ytsaurus-metrics"
	CRIServiceMonitoringPortName  = "cri-metrics"
)

const (
	YTMonitoringPort = 10000

	DiscoveryRPCPort        = 9020
	DiscoveryMonitoringPort = 10020

	MasterRPCPort        = 9010
	MasterMonitoringPort = 10010

	SchedulerRPCPort        = 9011
	SchedulerMonitoringPort = 10011

	ControllerAgentRPCPort        = 9014
	ControllerAgentMonitoringPort = 10014

	DataNodeRPCPort        = 9012
	DataNodeMonitoringPort = 10012

	TabletNodeRPCPort        = 9022
	TabletNodeMonitoringPort = 10022

	ExecNodeRPCPort        = 9029
	ExecNodeMonitoringPort = 10029

	CRIServiceMonitoringPort = 10026

	// TODO(zlobober): temporary until YT-20036.
	DataNodeSkynetPort   = 11012
	TabletNodeSkynetPort = 11022
	ExecNodeSkynetPort   = 11029

	RPCProxyPublicRPCPort   = 9013
	RPCProxyInternalRPCPort = 9023
	RPCProxyMonitoringPort  = 10013

	HTTPProxyRPCPort        = 9016
	HTTPProxyMonitoringPort = 10016
	HTTPProxyHTTPPort       = 80
	HTTPProxyHTTPSPort      = 443

	TCPProxyMonitoringPort = 10017

	QueryTrackerRPCPort        = 9028
	QueryTrackerMonitoringPort = 10028

	YQLAgentRPCPort        = 9019
	YQLAgentMonitoringPort = 10019

	QueueAgentRPCPort        = 9030
	QueueAgentMonitoringPort = 10030

	UIHTTPPort = 80

	StrawberryHTTPAPIPort = 80

	MasterCachesRPCPort        = 9018
	MasterCachesMonitoringPort = 10018

	KafkaProxyMonitoringPort = 10033
	KafkaProxyKafkaPort      = 9034
	KafkaProxyRPCPort        = 9033
)
