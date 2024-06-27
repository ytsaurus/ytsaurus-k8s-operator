package ytconfig

import (
	"path"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

func GetContainerdSocketPath(spec *ytv1.ExecNodesSpec) string {
	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeImageCache); location != nil {
		return path.Join(location.Path, consts.ContainerdSocketName)
	}
	// In non-overlayfs setup CRI could work without own location.
	return path.Join(consts.ConfigMountPoint, consts.ContainerdSocketName)
}

func (g *NodeGenerator) GetContainerdConfig(spec *ytv1.ExecNodesSpec) ([]byte, error) {
	criSpec := spec.JobEnvironment.CRI

	var rootPath *string
	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeImageCache); location != nil {
		rootPath = &location.Path
	}

	config := map[string]any{
		"version": 2,
		"root":    rootPath,

		"grpc": map[string]any{
			"address": GetContainerdSocketPath(spec),
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

	// TODO(khlebnikov): Refactor and remove this mess with formats.
	return marshallYsonConfig(config)
}
