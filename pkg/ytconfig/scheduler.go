package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type OperationsCleaner struct {
	EnableOperationArchivation *bool `yson:"enable_operation_archivation,omitempty"`
}

type Scheduler struct {
	OperationsCleaner OperationsCleaner `yson:"operations_cleaner"`
}

type SchedulerServer struct {
	CommonServer
	Scheduler Scheduler `yson:"scheduler"`
}

type ControllerAgent struct {
	EnableTmpfs bool `yson:"enable_tmpfs"`
}

type ControllerAgentServer struct {
	CommonServer
	ControllerAgent ControllerAgent `yson:"controller_agent"`
}

func getSchedulerServerCarcass(spec ytv1.SchedulersSpec) (SchedulerServer, error) {
	var c SchedulerServer
	c.RPCPort = consts.SchedulerRPCPort
	c.MonitoringPort = consts.SchedulerMonitoringPort

	c.Logging = createLogging(&spec.InstanceSpec, "scheduler", []ytv1.LoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})

	return c, nil
}

func getControllerAgentServerCarcass(spec ytv1.ControllerAgentsSpec) (ControllerAgentServer, error) {
	var c ControllerAgentServer
	c.RPCPort = consts.ControllerAgentRPCPort
	c.MonitoringPort = consts.ControllerAgentMonitoringPort

	c.Logging = createLogging(&spec.InstanceSpec, "controller-agent", []ytv1.LoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})

	return c, nil
}
