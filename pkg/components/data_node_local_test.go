package components

import (
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func TestDataNodeFlow(t *testing.T) {
	testComponentFlow(
		t,
		"dnd",
		"data-node",
		"-dn-1",
		func(_ *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), domain)
			return NewDataNode(
				cfgen,
				ytsaurus,
				nil,
				ytsaurus.GetResource().Spec.DataNodes[0],
			)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.DataNodes[0].Image = image
		},
	)
}
