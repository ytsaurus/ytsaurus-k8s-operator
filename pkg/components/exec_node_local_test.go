package components

import (
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func TestExecNodeFlow(t *testing.T) {
	testComponentFlow(
		t,
		"end",
		"exec-node",
		"",
		func(_ *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), domain)
			return NewExecNode(
				cfgen,
				ytsaurus,
				nil,
				ytsaurus.GetResource().Spec.ExecNodes[0],
			)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.ExecNodes[0].Image = image
		},
	)
}
