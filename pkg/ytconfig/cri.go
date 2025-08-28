package ytconfig

import (
	"fmt"
	"path"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type CRIConfigGenerator struct {
	Service        ytv1.CRIServiceType
	Spec           ytv1.CRIJobEnvironmentSpec
	Isolated       bool
	StoragePath    *string
	MonitoringPort int32
	HasGPU         bool
}

func NewCRIConfigGenerator(spec *ytv1.ExecNodesSpec) *CRIConfigGenerator {
	envSpec := spec.JobEnvironment
	if envSpec == nil || envSpec.CRI == nil {
		return &CRIConfigGenerator{
			Service: ytv1.CRIServiceNone,
		}
	}
	criSpec := envSpec.CRI
	config := &CRIConfigGenerator{
		Spec:           *criSpec,
		Service:        ptr.Deref(criSpec.CRIService, ytv1.CRIServiceContainerd),
		Isolated:       ptr.Deref(envSpec.Isolated, true),
		MonitoringPort: ptr.Deref(criSpec.MonitoringPort, consts.CRIServiceMonitoringPort),
		HasGPU:         resourceListHasGPU(spec.JobResources.Requests) || resourceListHasGPU(spec.JobResources.Limits),
	}
	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeImageCache); location != nil {
		config.StoragePath = &location.Path
	}
	return config
}

func (cri *CRIConfigGenerator) GetSocketPath() string {
	socketName := consts.CRIServiceSocketName
	if cri.Service == ytv1.CRIServiceContainerd {
		socketName = consts.ContainerdSocketName
	}
	if cri.StoragePath != nil {
		return path.Join(*cri.StoragePath, socketName)
	}
	// In non-overlayfs setup CRI could work without own location.
	return path.Join(consts.ConfigMountPoint, socketName)
}

func (cri *CRIConfigGenerator) GetCRIToolsEnv() []corev1.EnvVar {
	var env []corev1.EnvVar
	switch cri.Service {
	case ytv1.CRIServiceContainerd:
		// ctr
		env = append(env, corev1.EnvVar{Name: "CONTAINERD_ADDRESS", Value: cri.GetSocketPath()})
		env = append(env, corev1.EnvVar{Name: "CONTAINERD_NAMESPACE", Value: "k8s.io"})
		fallthrough
	case ytv1.CRIServiceCRIO:
		// crictl
		env = append(env, corev1.EnvVar{Name: "CONTAINER_RUNTIME_ENDPOINT", Value: "unix://" + cri.GetSocketPath()})
	}
	return env
}

func (cri *CRIConfigGenerator) GetCRIOEnv() []corev1.EnvVar {
	var env []corev1.EnvVar

	// See https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md
	env = append(env,
		corev1.EnvVar{Name: "CONTAINER_LISTEN", Value: cri.GetSocketPath()},
		corev1.EnvVar{Name: "CONTAINER_CGROUP_MANAGER", Value: "cgroupfs"},
		corev1.EnvVar{Name: "CONTAINER_CONMON_CGROUP", Value: "pod"},
	)
	if cri.StoragePath != nil {
		env = append(env, corev1.EnvVar{Name: "CONTAINER_ROOT", Value: *cri.StoragePath})
	}
	if cri.Spec.SandboxImage != nil {
		env = append(env, corev1.EnvVar{Name: "CONTAINER_PAUSE_IMAGE", Value: *cri.Spec.SandboxImage})
	}
	if cri.MonitoringPort != 0 {
		env = append(env,
			corev1.EnvVar{Name: "CONTAINER_ENABLE_METRICS", Value: "true"},
			corev1.EnvVar{Name: "CONTAINER_METRICS_HOST", Value: ""},
			corev1.EnvVar{Name: "CONTAINER_METRICS_PORT", Value: fmt.Sprintf("%d", cri.MonitoringPort)},
		)
	}
	return env
}

func (cri *CRIConfigGenerator) GetContainerdConfig() ([]byte, error) {
	runtimes, defaultRuntimeName := cri.defineContainerdRuntimes()

	// See https://github.com/containerd/containerd/blob/main/docs/cri/config.md
	config := map[string]any{
		"version": 2,
		"root":    cri.StoragePath,

		"grpc": map[string]any{
			"address": cri.GetSocketPath(),
			"uid":     0,
			"gid":     0,
		},

		"plugins": map[string]any{
			"io.containerd.grpc.v1.cri": map[string]any{
				"sandbox_image":               cri.Spec.SandboxImage,
				"restrict_oom_score_adj":      true,
				"image_pull_progress_timeout": "5m0s",

				"cni": map[string]any{
					"conf_dir": "/etc/cni/net.d",
					"bin_dir":  "/usr/local/lib/cni",
				},

				"containerd": map[string]any{
					"default_runtime_name": defaultRuntimeName,
					"runtimes":             runtimes,
				},

				"registry": map[string]any{
					"config_path": cri.Spec.RegistryConfigPath,
				},
			},
		},
	}

	if cri.MonitoringPort != 0 {
		config["metrics"] = map[string]any{
			"address": fmt.Sprintf(":%d", cri.MonitoringPort),
		}
	}

	// TODO(khlebnikov): Refactor and remove this mess with formats.
	return marshallYsonConfig(config)
}

func (cri *CRIConfigGenerator) defineContainerdRuntimes() (runtimes map[string]any, defaultRuntimeName string) {
	runtimes = map[string]any{
		"runc": map[string]any{
			"runtime_type": "io.containerd.runc.v2",
			"sandbox_mode": "podsandbox",
			"options": map[string]any{
				"SystemdCgroup": false,
			},
		},
	}
	defaultRuntimeName = "runc"

	if cri.HasGPU {
		runtimes["nvidia"] = map[string]any{
			"runtime_type": "io.containerd.nvidia.v1",
			"sandbox_mode": "podsandbox",
			"options": map[string]any{
				"BinaryName": "/usr/bin/nvidia-container-runtime",
			},
		}
		defaultRuntimeName = "nvidia"
	}

	return runtimes, defaultRuntimeName
}

func resourceListHasGPU(resourceList corev1.ResourceList) bool {
	for resource := range resourceList {
		if resource == "nvidia.com/gpu" {
			return true
		}
	}
	return false
}
