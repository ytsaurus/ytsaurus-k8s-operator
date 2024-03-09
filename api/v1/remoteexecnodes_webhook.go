package v1

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/library/go/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Defaulter = (*RemoteExecNodes)(nil)

func (r *RemoteExecNodes) Default() {
	r.setMonitoringPortDefaults()
}

func (r *RemoteExecNodes) setMonitoringPortDefaults() {
	if r.Spec.MonitoringPort == nil {
		r.Spec.MonitoringPort = ptr.Int32(consts.ExecNodeMonitoringPort)
	}
}
