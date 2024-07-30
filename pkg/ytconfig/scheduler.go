package ytconfig

import (
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
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
	EnableTmpfs                  bool `yson:"enable_tmpfs"`
	UseColumnarStatisticsDefault bool `yson:"use_columnar_statistics_default"`
}

type ControllerAgentServer struct {
	CommonServer
	ControllerAgent ControllerAgent `yson:"controller_agent"`
}

func getSchedulerLogging(spec *ytv1.SchedulersSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"scheduler",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getControllerAgentLogging(spec *ytv1.ControllerAgentsSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"controller-agent",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getSchedulerServerCarcass(spec *ytv1.SchedulersSpec) (SchedulerServer, error) {
	var c SchedulerServer
	c.RPCPort = consts.SchedulerRPCPort
	c.MonitoringPort = *spec.InstanceSpec.MonitoringPort
	c.Logging = getSchedulerLogging(spec)

	return c, nil
}

func getControllerAgentServerCarcass(spec *ytv1.ControllerAgentsSpec) (ControllerAgentServer, error) {
	var c ControllerAgentServer
	c.RPCPort = consts.ControllerAgentRPCPort
	c.MonitoringPort = *spec.InstanceSpec.MonitoringPort
	c.Logging = getControllerAgentLogging(spec)

	return c, nil
}
