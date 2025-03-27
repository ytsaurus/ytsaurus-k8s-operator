package ytconfig

import (
	"fmt"
	"path"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

func GetCRIServiceType(spec *ytv1.ExecNodesSpec) ytv1.CRIServiceType {
	if envSpec := spec.JobEnvironment; envSpec != nil && envSpec.CRI != nil {
		return ptr.Deref(envSpec.CRI.CRIService, ytv1.CRIServiceContainerd)
	}
	return ytv1.CRIServiceNone
}

func GetImageCacheLocationPath(spec *ytv1.ExecNodesSpec) *string {
	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeImageCache); location != nil {
		return &location.Path
	}
	return nil
}

func GetCRIServiceSocketPath(spec *ytv1.ExecNodesSpec) string {
	var socketName string
	switch GetCRIServiceType(spec) {
	case ytv1.CRIServiceContainerd:
		socketName = consts.ContainerdSocketName
	default:
		socketName = consts.CRIServiceSocketName
	}
	if locationPath := GetImageCacheLocationPath(spec); locationPath != nil {
		return path.Join(*locationPath, socketName)
	}
	// In non-overlayfs setup CRI could work without own location.
	return path.Join(consts.ConfigMountPoint, socketName)
}

func GetCRIServiceMonitoringPort(spec *ytv1.ExecNodesSpec) int32 {
	if envSpec := spec.JobEnvironment; envSpec != nil && envSpec.CRI != nil {
		return ptr.Deref(envSpec.CRI.MonitoringPort, consts.CRIServiceMonitoringPort)
	}
	return 0
}

func (g *NodeGenerator) GetContainerdConfig(spec *ytv1.ExecNodesSpec) ([]byte, error) {
	criSpec := spec.JobEnvironment.CRI

	config := map[string]any{
		"version": 2,
		"root":    GetImageCacheLocationPath(spec),

		"grpc": map[string]any{
			"address": GetCRIServiceSocketPath(spec),
			"uid":     0,
			"gid":     0,
		},

		"plugins": map[string]any{
			"io.containerd.grpc.v1.cri": map[string]any{
				"sandbox_image":               criSpec.SandboxImage,
				"restrict_oom_score_adj":      true,
				"image_pull_progress_timeout": "5m0s",

				"cni": map[string]any{
					"conf_dir": "/etc/cni/net.d",
					"bin_dir":  "/usr/local/lib/cni",
				},

				"containerd": map[string]any{
					"default_runtime_name": "runc",
					"runtimes": map[string]any{
						"runc": map[string]any{
							"runtime_type": "io.containerd.runc.v2",
							"sandbox_mode": "podsandbox",
							"options": map[string]any{
								"SystemdCgroup": false,
							},
						},
					},
				},

				"registry": map[string]any{
					"config_path": criSpec.RegistryConfigPath,
				},
			},
		},
	}

	if port := GetCRIServiceMonitoringPort(spec); port != 0 {
		config["metrics"] = map[string]any{
			"address": fmt.Sprintf(":%d", port),
		}
	}

	// TODO(khlebnikov): Refactor and remove this mess with formats.
	return marshallYsonConfig(config)
}
