package components

import (
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func NewMasterExitReadOnlyJob(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *JobStateless {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelMaster,
		ComponentName:  "MasterExitReadOnlyJob",
		MonitoringPort: consts.MasterMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	return NewJobStateless(
		&l,
		ytsaurus.APIProxy(),
		resource.Spec.ImagePullSecrets,
		"exit-read-only",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig,
	)
}

func CreateExitReadOnlyScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		"export YT_LOG_LEVEL=DEBUG",
		// COMPAT(l0kix2): remove || part when the compatibility with 23.1 and older is dropped.
		`[[ "$YTSAURUS_VERSION" < "23.2" ]] && echo "master_exit_read_only is supported since 23.2, nothing to do" && exit 0`,
		"/usr/bin/yt execute master_exit_read_only '{}'",
	}

	return strings.Join(script, "\n")
}
