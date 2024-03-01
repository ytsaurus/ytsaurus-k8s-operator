package components

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func NewInitOpArchiveJob(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *JobStateless {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelScheduler,
		ComponentName:  "InitOpArchiveJob",
		MonitoringPort: consts.SchedulerMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}
	return NewJobStateless(
		&l,
		ytsaurus.APIProxy(),
		resource.Spec.ImagePullSecrets,
		"user",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig,
	)
}

func NewUpdateOpArchiveJob(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *JobStateless {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelScheduler,
		ComponentName:  "UpdateOpArchiveJob",
		MonitoringPort: consts.SchedulerMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}
	return NewJobStateless(
		&l,
		ytsaurus.APIProxy(),
		resource.Spec.ImagePullSecrets,
		"op-archive",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig,
	)
}
