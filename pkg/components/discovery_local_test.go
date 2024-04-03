package components

import (
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func TestDiscoveryFlow(t *testing.T) {
	testComponentFlow(
		t,
		"ds",
		"discovery",
		"",
		func(cfgen *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			return NewDiscovery(cfgen, ytsaurus)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.Discovery.Image = image
		},
	)
}
