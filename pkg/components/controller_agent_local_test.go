package components

import (
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func TestControllerAgentFlow(t *testing.T) {
	testComponentFlow(
		t,
		"ca",
		"controller-agent",
		"",
		func(cfgen *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			return NewControllerAgent(
				cfgen,
				ytsaurus,
				nil,
			)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.ControllerAgents.Image = image
		},
	)
}
