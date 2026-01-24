package consts

import (
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type ComponentType = ytv1.ComponentType

const (
	BundleControllerType     = ytv1.BundleControllerType
	TabletBalancerType       = ytv1.TabletBalancerType
	ControllerAgentType      = ytv1.ControllerAgentType
	CypressProxyType         = ytv1.CypressProxyType
	DataNodeType             = ytv1.DataNodeType
	DiscoveryType            = ytv1.DiscoveryType
	ExecNodeType             = ytv1.ExecNodeType
	HttpProxyType            = ytv1.HttpProxyType
	MasterCacheType          = ytv1.MasterCacheType
	MasterType               = ytv1.MasterType
	QueryTrackerType         = ytv1.QueryTrackerType
	QueueAgentType           = ytv1.QueueAgentType
	RpcProxyType             = ytv1.RpcProxyType
	SchedulerType            = ytv1.SchedulerType
	StrawberryControllerType = ytv1.StrawberryControllerType
	TabletNodeType           = ytv1.TabletNodeType
	OffshoreDataGatewayType  = ytv1.OffshoreDataGatewayType
	TcpProxyType             = ytv1.TcpProxyType
	KafkaProxyType           = ytv1.KafkaProxyType
	UIType                   = ytv1.UIType
	YqlAgentType             = ytv1.YqlAgentType
	YtsaurusClientType       = ytv1.YtsaurusClientType
	ChytType                 = ytv1.ChytType
	SpytType                 = ytv1.SpytType
	ClusterConnectionType    = ytv1.ClusterConnectionType
	NativeClientConfigType   = ytv1.NativeClientConfigType
	TimbertruckType          = ytv1.TimbertruckType
	ImageHeaterType          = ytv1.ImageHeaterType
)

type ComponentClass = ytv1.ComponentClass

const (
	ComponentClassUnspecified = ytv1.ComponentClassUnspecified
	ComponentClassStateless   = ytv1.ComponentClassStateless
	ComponentClassEverything  = ytv1.ComponentClassEverything
	ComponentClassNothing     = ytv1.ComponentClassNothing
)

var (
	LocalComponentTypes = []ComponentType{
		BundleControllerType,
		TabletBalancerType,
		ControllerAgentType,
		DataNodeType,
		DiscoveryType,
		ExecNodeType,
		HttpProxyType,
		MasterCacheType,
		MasterType,
		QueryTrackerType,
		QueueAgentType,
		RpcProxyType,
		SchedulerType,
		StrawberryControllerType,
		TabletNodeType,
		TcpProxyType,
		KafkaProxyType,
		YqlAgentType,
		ClusterConnectionType,
		NativeClientConfigType,
		ImageHeaterType,
	}

	RemoteComponentTypes = []ComponentType{
		DataNodeType,
		ExecNodeType,
		TabletNodeType,
		OffshoreDataGatewayType,
	}
)
