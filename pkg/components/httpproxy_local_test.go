package components

import (
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func TestHttpproxyFlow(t *testing.T) {
	testComponentFlow(
		t,
		"hp",
		"http-proxy",
		"",
		func(cfgen *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			return NewHTTPProxy(
				cfgen,
				ytsaurus,
				nil,
				ytsaurus.GetResource().Spec.HTTPProxies[0],
			)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.HTTPProxies[0].Image = image
		},
	)
}
