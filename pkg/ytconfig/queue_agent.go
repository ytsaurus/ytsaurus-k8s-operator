package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type QueueAgent struct {
	Stage string `yson:"stage"`
}

type QueueAgentServer struct {
	CommonServer

	BusClient  *Bus       `yson:"bus_client,omitempty"`
	User       string     `yson:"user"`
	QueueAgent QueueAgent `yson:"queue_agent"`
}

func getQueueAgentLogging(spec *ytv1.QueueAgentSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"queue-agent",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getQueueAgentServerCarcass(spec *ytv1.QueueAgentSpec) (QueueAgentServer, error) {
	var c QueueAgentServer
	c.RPCPort = consts.QueueAgentRPCPort

	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.QueueAgentMonitoringPort)
	c.User = "queue_agent"
	c.QueueAgent.Stage = "production"

	c.Logging = getQueueAgentLogging(spec)

	return c, nil
}
