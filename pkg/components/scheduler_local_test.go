package components

import (
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func TestSchedulerFlow(t *testing.T) {
	testComponentFlow(
		t,
		"sch",
		"scheduler",
		"",
		func(cfgen *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			return NewScheduler(cfgen, ytsaurus, nil, nil, nil)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.Schedulers.Image = image
		},
	)
}
