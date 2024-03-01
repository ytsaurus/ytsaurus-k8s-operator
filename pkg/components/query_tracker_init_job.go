package components

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func NewInitQueryTrackerJob(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *JobStateless {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: "yt-query-tracker",
		ComponentName:  "InitQueryTrackerJob",
		MonitoringPort: consts.QueryTrackerMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}
	image := ytsaurus.GetResource().Spec.CoreImage
	if resource.Spec.QueryTrackers.InstanceSpec.Image != nil {
		image = *resource.Spec.QueryTrackers.InstanceSpec.Image
	}
	return NewJobStateless(
		&l,
		ytsaurus.APIProxy(),
		resource.Spec.ImagePullSecrets,
		"qt-state",
		consts.ClientConfigFileName,
		image,
		cfgen.GetNativeClientConfig,
	)
}
