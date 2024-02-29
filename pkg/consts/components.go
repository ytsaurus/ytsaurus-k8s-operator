package consts

type ComponentType string

const (
	ComponentTypeDiscovery       ComponentType = "Discovery"
	ComponentTypeMaster          ComponentType = "Master"
	ComponentTypeScheduler       ComponentType = "Scheduler"
	ComponentTypeControllerAgent ComponentType = "ControllerAgent"
	ComponentTypeDataNode        ComponentType = "DataNode"
	ComponentTypeExecNode        ComponentType = "ExecNode"
	ComponentTypeTabletNode      ComponentType = "TabletNode"
	ComponentTypeHTTPProxy       ComponentType = "HttpProxy"
	ComponentTypeRPCProxy        ComponentType = "RpcProxy"
	ComponentTypeTCPProxy        ComponentType = "TcpProxy"
	ComponentTypeUI              ComponentType = "UI"
	ComponentTypeYqlAgent        ComponentType = "YqlAgent"
	ComponentTypeClient          ComponentType = "YtsaurusClient"
)

//// AllComponentTypes must be in sync with list of types.
//var AllComponentTypes = []ComponentType{
//	ComponentTypeDiscovery,
//	ComponentTypeMaster,
//	ComponentTypeScheduler,
//	ComponentTypeControllerAgent,
//	ComponentTypeDataNode,
//	ComponentTypeExecNode,
//	ComponentTypeTabletNode,
//	ComponentTypeHTTPProxy,
//	ComponentTypeRPCProxy,
//	ComponentTypeTCPProxy,
//	ComponentTypeUI,
//	ComponentTypeYqlAgent,
//	ComponentTypeClient,
//}
