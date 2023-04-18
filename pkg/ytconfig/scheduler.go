package ytconfig

import (
	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
)

type OperationsCleaner struct {
	EnableOperationArchivation bool  `yson:"enable_operation_archivation"`
	CleanDelay                 int64 `yson:"clean_delay"`
}

type Scheduler struct {
	OperationsCleaner          OperationsCleaner `yson:"operations_cleaner"`
	SendRegisteredAgentsToNode bool              `yson:"send_registered_agents_to_node"`
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

	c.Scheduler.OperationsCleaner.EnableOperationArchivation = false
	c.Scheduler.OperationsCleaner.CleanDelay = 24 * 60 * 60 * 1000
	c.Scheduler.SendRegisteredAgentsToNode = true

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "scheduler")
	c.Logging = loggingBuilder.addDefaultInfo().addDefaultStderr().logging

	return c, nil
}

func getControllerAgentServerCarcass(spec ytv1.ControllerAgentsSpec) (ControllerAgentServer, error) {
	var c ControllerAgentServer
	c.RPCPort = consts.ControllerAgentRPCPort
	c.MonitoringPort = consts.ControllerAgentMonitoringPort

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "controller-agent")
	c.Logging = loggingBuilder.addDefaultInfo().addDefaultStderr().logging

	return c, nil
}
