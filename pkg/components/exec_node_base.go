package components

import (
	"context"
	"log"
	"path"

	corev1 "k8s.io/api/core/v1"
	ptr "k8s.io/utils/pointer" //nolint:staticcheck

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type baseExecNode struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.ExecNodesSpec

	sidecarConfig *ConfigHelper
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
			ct.SecurityContext.Privileged = ptr.Bool(n.spec.Privileged)
		}
	}

	for i := range podSpec.InitContainers {
		setContainerPrivileged(&podSpec.InitContainers[i])
	}

	for i := range podSpec.Containers {
		setContainerPrivileged(&podSpec.Containers[i])

		if envSpec := n.spec.JobEnvironment; envSpec != nil && envSpec.CRI != nil {
			n.addEnvironmentForCRITools(&podSpec.Containers[i])
		}
	}

	// Pour job resources into node container if jobs are not isolated.
	if n.spec.JobResources != nil && !n.IsJobEnvironmentIsolated() {
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

		addResourceList(podSpec.Containers[0].Resources.Requests, n.spec.JobResources.Requests)
		addResourceList(podSpec.Containers[0].Resources.Limits, n.spec.JobResources.Limits)
	}

	if n.IsJobEnvironmentIsolated() {
		// Add sidecar container for running jobs.
		if envSpec := n.spec.JobEnvironment; envSpec != nil && envSpec.CRI != nil {
			n.doBuildCRISidecar(envSpec, podSpec)
		}
	} else if n.sidecarConfig != nil {
		// Mount sidecar config into exec node container if job environment is not isolated.
		// CRI service is supposed to be started by exec node entrypoint wrapper.
		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      consts.ContainerdConfigVolumeName,
				MountPath: consts.ContainerdConfigMountPoint,
				ReadOnly:  true,
			})
	}

	if n.sidecarConfig != nil {
		podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(consts.ContainerdConfigVolumeName,
			n.sidecarConfig.labeller.GetSidecarConfigMapName(consts.JobsContainerName), nil))

		n.sidecarConfig.Build()
	}

	return nil
}

func (n *baseExecNode) addEnvironmentForCRITools(container *corev1.Container) {
	socketPath := ytconfig.GetContainerdSocketPath(n.spec)
	container.Env = append(container.Env, []corev1.EnvVar{
		{Name: "CONTAINERD_ADDRESS", Value: socketPath},                     // ctr
		{Name: "CONTAINERD_NAMESPACE", Value: "k8s.io"},                     // ctr
		{Name: "CONTAINER_RUNTIME_ENDPOINT", Value: "unix://" + socketPath}, // crictl
	}...)
}

func (n *baseExecNode) doBuildCRISidecar(envSpec *ytv1.JobEnvironmentSpec, podSpec *corev1.PodSpec) {
	configPath := path.Join(consts.ContainerdConfigMountPoint, consts.ContainerdConfigFileName)

	wrapper := envSpec.CRI.EntrypointWrapper
	if len(wrapper) == 0 {
		wrapper = []string{"tini", "--"}
	}
	command := make([]string, 0, 1+len(wrapper))
	command = append(append(command, wrapper...), "containerd")

	jobsContainer := corev1.Container{
		Name:         consts.JobsContainerName,
		Image:        podSpec.Containers[0].Image,
		Command:      command,
		Args:         []string{"--config", configPath},
		VolumeMounts: createVolumeMounts(n.spec.VolumeMounts),
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.Bool(true),
		},
	}

	n.addEnvironmentForCRITools(&jobsContainer)

	jobsContainer.VolumeMounts = append(jobsContainer.VolumeMounts,
		corev1.VolumeMount{
			Name:      consts.ContainerdConfigVolumeName,
			MountPath: consts.ContainerdConfigMountPoint,
			ReadOnly:  true,
		})

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
