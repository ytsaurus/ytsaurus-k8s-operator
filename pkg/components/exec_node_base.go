package components

import (
	"context"
	"fmt"
	"log"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type baseExecNode struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.ExecNodesSpec

	sidecarConfig *ConfigMapBuilder
}

// Returns true if jobs are executed outside of exec node container.
func (n *baseExecNode) IsJobEnvironmentIsolated() bool {
	if envSpec := n.spec.JobEnvironment; envSpec != nil {
		if envSpec.Isolated != nil {
			return *envSpec.Isolated
		}
		if n.spec.JobEnvironment.CRI != nil {
			return true
		}
	}
	return false
}

func (n *baseExecNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server, n.sidecarConfig)
}

func (n *baseExecNode) doBuildBase() error {
	statefulSet := n.server.buildStatefulSet()
	podSpec := &statefulSet.Spec.Template.Spec

	if len(podSpec.Containers) != 1 {
		log.Panicf("Number of exec node containers is expected to be 1, actual %v", len(podSpec.Containers))
	}

	if err := AddInitContainersToPodSpec(n.spec.InitContainers, podSpec); err != nil {
		return err
	}

	if err := AddSidecarsToPodSpec(n.spec.Sidecars, podSpec); err != nil {
		return err
	}

	setContainerPrivileged := func(ct *corev1.Container) {
		if ct.SecurityContext == nil {
			ct.SecurityContext = &corev1.SecurityContext{}
		}
		if ct.SecurityContext.Privileged == nil {
			ct.SecurityContext.Privileged = ptr.To(n.spec.Privileged)
		}
	}

	for i := range podSpec.InitContainers {
		setContainerPrivileged(&podSpec.InitContainers[i])
	}

	for i := range podSpec.Containers {
		setContainerPrivileged(&podSpec.Containers[i])
		n.addEnvironmentForTools(&podSpec.Containers[i])
	}

	if n.IsJobEnvironmentIsolated() {
		// Add CRI service sidecar container for running jobs.
		n.doBuildCRIServiceSidecar(podSpec)
	} else {
		// CRI service is supposed to be started by exec node entrypoint wrapper.
		n.doBuildCRIServiceInplace(&podSpec.Containers[0])
	}

	if n.sidecarConfig != nil {
		podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(consts.ContainerdConfigVolumeName,
			n.sidecarConfig.labeller.GetSidecarConfigMapName(consts.JobsContainerName), nil))

		n.sidecarConfig.Build()
	}

	return nil
}

func (n *baseExecNode) addCRIServicePorts(container *corev1.Container) {
	if port := ytconfig.GetCRIServiceMonitoringPort(n.spec); port != 0 {
		container.Ports = append(container.Ports, corev1.ContainerPort{
			Name:          consts.CRIServiceMonitoringPortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: port,
		})
	}
}

func (n *baseExecNode) addEnvironmentForTools(container *corev1.Container) {
	switch ytconfig.GetCRIServiceType(n.spec) {
	case ytv1.CRIServiceContainerd:
		// ctr
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "CONTAINERD_ADDRESS",
			Value: ytconfig.GetCRIServiceSocketPath(n.spec),
		}, corev1.EnvVar{
			Name:  "CONTAINERD_NAMESPACE",
			Value: "k8s.io",
		})
		fallthrough
	case ytv1.CRIServiceCRIO:
		// crictl
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "CONTAINER_RUNTIME_ENDPOINT",
			Value: "unix://" + ytconfig.GetCRIServiceSocketPath(n.spec),
		})
	}
}

func (n *baseExecNode) addEnvironmentForCRIO(criSpec *ytv1.CRIJobEnvironmentSpec, container *corev1.Container) {
	appendEnv := func(name, value string) {
		container.Env = append(container.Env, corev1.EnvVar{Name: name, Value: value})
	}

	// See https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md
	appendEnv("CONTAINER_LISTEN", ytconfig.GetCRIServiceSocketPath(n.spec))
	appendEnv("CONTAINER_CGROUP_MANAGER", "cgroupfs")
	appendEnv("CONTAINER_CONMON_CGROUP", "pod")
	if locationPath := ytconfig.GetImageCacheLocationPath(n.spec); locationPath != nil {
		appendEnv("CONTAINER_ROOT", *locationPath)
	}
	if criSpec.SandboxImage != nil {
		appendEnv("CONTAINER_PAUSE_IMAGE", *criSpec.SandboxImage)
	}
	if port := ytconfig.GetCRIServiceMonitoringPort(n.spec); port != 0 {
		appendEnv("CONTAINER_ENABLE_METRICS", "true")
		appendEnv("CONTAINER_METRICS_HOST", "")
		appendEnv("CONTAINER_METRICS_PORT", fmt.Sprintf("%d", port))
	}
}

func (n *baseExecNode) doBuildCRIServiceSidecar(podSpec *corev1.PodSpec) {
	criService := ytconfig.GetCRIServiceType(n.spec)
	if criService == ytv1.CRIServiceNone {
		return
	}
	envSpec := n.spec.JobEnvironment

	wrapper := envSpec.CRI.EntrypointWrapper
	if len(wrapper) == 0 {
		wrapper = []string{"tini", "--"}
	}

	command := make([]string, 0, 1+len(wrapper))
	command = append(command, wrapper...)
	command = append(command, string(criService))

	jobsContainer := corev1.Container{
		Name:         consts.JobsContainerName,
		Image:        podSpec.Containers[0].Image,
		Command:      command,
		Env:          getDefaultEnv(),
		VolumeMounts: createVolumeMounts(n.spec.VolumeMounts),
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(true),
		},
	}

	switch criService {
	case ytv1.CRIServiceContainerd:
		configPath := path.Join(consts.ContainerdConfigMountPoint, consts.ContainerdConfigFileName)
		jobsContainer.Args = []string{"--config", configPath}
		jobsContainer.VolumeMounts = append(jobsContainer.VolumeMounts,
			corev1.VolumeMount{
				Name:      consts.ContainerdConfigVolumeName,
				MountPath: consts.ContainerdConfigMountPoint,
				ReadOnly:  true,
			})
	case ytv1.CRIServiceCRIO:
		n.addEnvironmentForCRIO(envSpec.CRI, &jobsContainer)
	}

	n.addCRIServicePorts(&jobsContainer)
	n.addEnvironmentForTools(&jobsContainer)

	n.server.addCABundleMount(&jobsContainer)
	n.server.addTlsSecretMount(&jobsContainer)

	// Replace mount propagation "Bidirectional" -> "HostToContainer".
	// Tmpfs are propagated: exec-node -> host -> containerd.
	for i := range jobsContainer.VolumeMounts {
		mount := &jobsContainer.VolumeMounts[i]
		newProp := corev1.MountPropagationHostToContainer
		if prop := mount.MountPropagation; prop != nil && *prop == corev1.MountPropagationBidirectional {
			mount.MountPropagation = &newProp
		}
	}

	if n.spec.JobResources != nil {
		jobsContainer.Resources = *n.spec.JobResources.DeepCopy()
	} else {
		// Without dedicated job resources enforce same limits as for node.
		jobsContainer.Resources.Limits = n.spec.Resources.Limits
	}

	podSpec.Containers = append(podSpec.Containers, jobsContainer)
}

func (n *baseExecNode) doBuildCRIServiceInplace(container *corev1.Container) {
	// Pour job resources into node container if jobs are not isolated.
	if n.spec.JobResources != nil {
		addResourceList := func(list, newList corev1.ResourceList) {
			for name, quantity := range newList {
				if value, ok := list[name]; ok {
					value.Add(quantity)
					list[name] = value
				} else {
					list[name] = quantity.DeepCopy()
				}
			}
		}

		addResourceList(container.Resources.Requests, n.spec.JobResources.Requests)
		addResourceList(container.Resources.Limits, n.spec.JobResources.Limits)
	}

	criService := ytconfig.GetCRIServiceType(n.spec)
	if criService == ytv1.CRIServiceNone {
		return
	}
	envSpec := n.spec.JobEnvironment

	if n.sidecarConfig != nil {
		// Mount sidecar config into exec node container if job environment is not isolated.
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      consts.ContainerdConfigVolumeName,
			MountPath: consts.ContainerdConfigMountPoint,
			ReadOnly:  true,
		})
	}

	if criService == ytv1.CRIServiceCRIO {
		n.addEnvironmentForCRIO(envSpec.CRI, container)
	}

	n.addCRIServicePorts(container)
}

func (n *baseExecNode) doSyncBase(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	if !dry {
		err = n.doBuildBase()
		if err != nil {
			return WaitingStatus(SyncStatusBlocked, "cannot build exec node spec"), err
		}
		err = resources.Sync(ctx, n.server, n.sidecarConfig)
	}
	return WaitingStatus(SyncStatusPending, "components"), err
}

func (n *baseExecNode) sidecarConfigNeedsReload() bool {
	if n.sidecarConfig == nil {
		return false
	}

	needsReload, err := n.sidecarConfig.NeedReload()
	if err != nil {
		return false
	}
	return needsReload
}
