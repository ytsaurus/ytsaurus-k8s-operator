package ytconfig

import (
	"fmt"
	"path"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

const (
	runtimeTypeOCI = "oci"

	runtimeNameRunc     = "runc"
	crioRuntimePathRunc = "/usr/libexec/crio/runc"

	runtimeNameCrun     = "crun"
	crioRuntimePathCrun = "/usr/libexec/crio/crun"

	runtimeNameNvidia = "nvidia"
	runtimePathNvidia = "/usr/bin/nvidia-container-runtime"

	crioMonitorCgroup = "pod"
	crioMonitorPath   = "/usr/libexec/crio/conmon"

	shmSizeAnnotation = "io.kubernetes.cri-o.ShmSize"
)

type CRIConfigGenerator struct {
	Service        ytv1.CRIServiceType
	Spec           ytv1.CRIJobEnvironmentSpec
	Runtime        *ytv1.JobRuntimeSpec
	Isolated       bool
	StoragePath    *string
	MonitoringPort int32
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
		Runtime:        envSpec.Runtime,
		Service:        ptr.Deref(criSpec.CRIService, ytv1.CRIServiceContainerd),
		Isolated:       ptr.Deref(envSpec.Isolated, true),
		MonitoringPort: ptr.Deref(criSpec.MonitoringPort, consts.CRIServiceMonitoringPort),
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

func (cri *CRIConfigGenerator) GetCRIOSignaturePolicy() ([]byte, error) {
	// Allow any container images.
	policy := map[string]any{
		"default": []map[string]any{
			{
				"type": "insecureAcceptAnything",
			},
		},
	}
	return marshallYsonConfig(policy)
}

func (cri *CRIConfigGenerator) GetCRIOConfig() ([]byte, error) {
	// See https://github.com/cri-o/cri-o/blob/main/docs/crio.conf.5.md

	crioAPI := map[string]any{
		"listen": cri.GetSocketPath(),
	}

	crioImage := map[string]any{
		"signature_policy": path.Join(consts.CRIOConfigMountPoint, consts.CRIOSignaturePolicyFileName),
	}

	crioMetrics := map[string]any{}

	crioRuntimeRuntimes := map[string]any{
		runtimeNameRunc: map[string]any{
			"runtime_type": runtimeTypeOCI,
			"runtime_path": crioRuntimePathRunc,
			"allowed_annotations": []string{
				shmSizeAnnotation,
			},
			"monitor_cgroup": crioMonitorCgroup,
			"monitor_path":   crioMonitorPath,
		},
		runtimeNameCrun: map[string]any{
			"runtime_type": runtimeTypeOCI,
			"runtime_path": crioRuntimePathCrun,
			"allowed_annotations": []string{
				shmSizeAnnotation,
			},
			"monitor_cgroup": crioMonitorCgroup,
			"monitor_path":   crioMonitorPath,
		},
	}

	crioRuntime := map[string]any{
		"cgroup_manager":  "cgroupfs",
		"conmon_cgroup":   crioMonitorCgroup,
		"default_runtime": runtimeNameCrun,
		"runtimes":        crioRuntimeRuntimes,
	}

	crio := map[string]any{
		"api":     crioAPI,
		"image":   crioImage,
		"metrics": crioMetrics,
		"runtime": crioRuntime,
	}

	config := map[string]any{
		"crio": crio,
	}

	if cri.StoragePath != nil {
		crio["root"] = *cri.StoragePath
	}

	if cri.Spec.SandboxImage != nil {
		crioImage["pause_image"] = *cri.Spec.SandboxImage
	}

	if cri.MonitoringPort != 0 {
		crioMetrics["enable_metrics"] = true
		crioMetrics["metrics_host"] = ""
		crioMetrics["metrics_port"] = cri.MonitoringPort
	}

	if cri.Runtime != nil && cri.Runtime.Nvidia != nil {
		crioRuntimeRuntimes[runtimeNameNvidia] = map[string]any{
			"runtime_type": runtimeTypeOCI,
			"runtime_path": runtimePathNvidia,
			"allowed_annotations": []string{
				shmSizeAnnotation,
			},
			"monitor_cgroup": crioMonitorCgroup,
			"monitor_path":   crioMonitorPath,
		}
		crioRuntime["default_runtime"] = runtimeNameNvidia
	}

	return marshallYsonConfig(config)
}

func (cri *CRIConfigGenerator) GetContainerdConfig() ([]byte, error) {
	runtimes, defaultRuntimeName := cri.getContainerdRuntimes()

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

func (cri *CRIConfigGenerator) getContainerdRuntimes() (runtimes map[string]any, defaultRuntimeName string) {
	runtimes = map[string]any{
		runtimeNameRunc: map[string]any{
			"runtime_type": "io.containerd.runc.v2",
			"sandbox_mode": "podsandbox",
			"options": map[string]any{
				"SystemdCgroup": false,
			},
		},
	}
	defaultRuntimeName = runtimeNameRunc

	if cri.Runtime != nil && cri.Runtime.Nvidia != nil {
		runtimes[runtimeNameNvidia] = map[string]any{
			"runtime_type": "io.containerd.runc.v2",
			"sandbox_mode": "podsandbox",
			"options": map[string]any{
				"BinaryName": runtimePathNvidia,
			},
		}
		defaultRuntimeName = runtimeNameNvidia
	}

	return runtimes, defaultRuntimeName
}
