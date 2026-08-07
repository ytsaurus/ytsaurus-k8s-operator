package components

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createVolumeClaims(specVolumeClaimTemplates []ytv1.EmbeddedPersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	volumeClaims := make([]corev1.PersistentVolumeClaim, 0, len(specVolumeClaimTemplates))
	for _, volumeClaim := range specVolumeClaimTemplates {
		volumeClaims = append(volumeClaims, corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        volumeClaim.Name,
				Labels:      volumeClaim.Labels,
				Annotations: volumeClaim.Annotations,
			},
			Spec: *volumeClaim.Spec.DeepCopy(),
		})
	}
	return volumeClaims
}

func createVolumes(specVolumes []ytv1.Volume) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(specVolumes))
	for _, volume := range specVolumes {
		volumes = append(volumes, corev1.Volume{
			Name: volume.Name,
			VolumeSource: corev1.VolumeSource{
				HostPath:              volume.HostPath,
				EmptyDir:              volume.EmptyDir,
				Secret:                volume.Secret,
				NFS:                   volume.NFS,
				ISCSI:                 volume.ISCSI,
				PersistentVolumeClaim: volume.PersistentVolumeClaim,
				DownwardAPI:           volume.DownwardAPI,
				FC:                    volume.FC,
				ConfigMap:             volume.ConfigMap,
				CSI:                   volume.CSI,
				Ephemeral:             volume.Ephemeral,
				Image:                 volume.Image,
			},
		})
	}
	return volumes
}

func createConfigTemplateVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      consts.ConfigTemplateVolumeName,
		MountPath: consts.ConfigTemplateMountPoint,
		ReadOnly:  true,
	}
}

func createConfigVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      consts.ConfigVolumeName,
		MountPath: consts.ConfigMountPoint,
		ReadOnly:  false,
	}
}

func createVolumeMounts(specVolumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0, len(specVolumeMounts)+1)
	volumeMounts = append(volumeMounts, specVolumeMounts...)
	volumeMounts = append(volumeMounts, createConfigTemplateVolumeMount())
	volumeMounts = append(volumeMounts, createConfigVolumeMount())
	return volumeMounts
}

// FindVolumeMountForPath returns the volume mount that path resides in.
// Mounts are scanned backward: the kubelet applies mounts in order, so the
// last mount covering a path is the one visible in the container.
func FindVolumeMountForPath(volumeMounts []corev1.VolumeMount, path string) *corev1.VolumeMount {
	for i, mount := range slices.Backward(volumeMounts) {
		if path == mount.MountPath || strings.HasPrefix(path, mount.MountPath+"/") {
			return &volumeMounts[i]
		}
	}
	return nil
}

func resolveLocationMounts(
	instanceSpec *ytv1.InstanceSpec,
	requiredLocations ...ytv1.LocationType,
) ([]corev1.VolumeMount, error) {
	for _, requiredLocation := range requiredLocations {
		if ytv1.FindFirstLocation(instanceSpec.Locations, requiredLocation) == nil {
			return nil, fmt.Errorf("no location of type %q found", requiredLocation)
		}
	}
	mounts := []corev1.VolumeMount{}
	for _, location := range instanceSpec.Locations {
		if !slices.Contains(requiredLocations, location.LocationType) {
			continue
		}
		volumeMount := FindVolumeMountForPath(instanceSpec.VolumeMounts, location.Path)
		if volumeMount == nil {
			return nil, fmt.Errorf("no volume mount covers location %q (path %q)", location.LocationType, location.Path)
		}
		if slices.ContainsFunc(mounts, func(mount corev1.VolumeMount) bool { return mount.MountPath == location.Path }) {
			continue
		}
		relPath := strings.TrimPrefix(strings.TrimPrefix(location.Path, volumeMount.MountPath), "/")
		mount := corev1.VolumeMount{
			Name:      volumeMount.Name,
			MountPath: location.Path,
		}
		// SubPath and SubPathExpr are mutually exclusive.
		if volumeMount.SubPathExpr != "" {
			mount.SubPathExpr = path.Join(volumeMount.SubPathExpr, relPath)
		} else {
			mount.SubPath = path.Join(volumeMount.SubPath, relPath)
		}
		mounts = append(mounts, mount)
	}
	slices.SortStableFunc(mounts, func(a, b corev1.VolumeMount) int {
		return cmp.Compare(len(a.MountPath), len(b.MountPath))
	})
	return mounts, nil
}

func resolveVolumeMounts(
	instanceSpec *ytv1.InstanceSpec,
	volumeMounts []corev1.VolumeMount,
	pvcSuffix string,
) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(volumeMounts))
	for _, mnt := range volumeMounts {
		for _, vol := range instanceSpec.Volumes {
			if vol.Name == mnt.Name {
				volumes = append(volumes, createVolumes([]ytv1.Volume{vol})...)
			}
		}
		for _, vol := range instanceSpec.VolumeClaimTemplates {
			if vol.Name == mnt.Name {
				volumes = append(volumes, corev1.Volume{
					Name: vol.Name,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: vol.Name + pvcSuffix,
						},
					},
				})
			}
		}
	}
	return volumes
}

func createConfigVolume(volumeName string, configMapName string, mode *int32) corev1.Volume {
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				DefaultMode: mode,
			},
		},
	}
}

func createConfigEmptyDir() corev1.Volume {
	return corev1.Volume{
		Name: consts.ConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func createServerVolumes(specVolumes []ytv1.Volume, configMapName string) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(specVolumes)+1)
	volumes = append(volumes, createVolumes(specVolumes)...)

	volumes = append(volumes, createConfigVolume(consts.ConfigTemplateVolumeName, configMapName, nil))
	volumes = append(volumes, createConfigEmptyDir())
	return volumes
}

func getLocationInitCommand(locations []ytv1.LocationSpec) string {
	var command strings.Builder
	command.WriteString("echo 'Init locations'; ")
	for _, location := range locations {
		command.WriteString("mkdir -p ")
		command.WriteString(location.Path)
		command.WriteString("; ")
	}
	return command.String()
}

func getConfigPostprocessingCommand(configTemplatePaths ...string) string {
	var command strings.Builder

	// Store postprocessing as a script on filesystem to ease up manual
	// config re-initialization without pod recreation. This will be useful
	// when operator starts restarting processes without container recreation.

	postprocessScriptPath := path.Join(consts.ConfigMountPoint, consts.PostprocessConfigScriptName)

	var postprocessScript strings.Builder

	substituteEnvCommand := func(configPath, envVar string) {
		// Replace placeholder {envVar} with the actual value of environment variable envVar.
		fmt.Fprintf(&postprocessScript, "sed -i -s \"s/{%v}/${%v}/g\" %v; ", envVar, envVar, configPath)
	}

	substitutePlaceholderWithCommand := func(configPath, placeholder, command string) {
		// Replace placeholder {placeholder} with the output of the given command.
		fmt.Fprintf(&postprocessScript, "sed -i -s \"s/{%v}/$(%v)/g\" %v; ", placeholder, command, configPath)
	}

	for _, configTemplatePath := range configTemplatePaths {
		configFileName := path.Base(configTemplatePath)
		configPath := path.Join(consts.ConfigMountPoint, configFileName)

		fmt.Fprintf(&command, "echo 'Postprocess config %v';", configFileName)
		fmt.Fprintf(&postprocessScript, "cp %v %v; ", configTemplatePath, configPath)

		for _, envVar := range getDefaultEnv() {
			substituteEnvCommand(configPath, envVar.Name)
		}

		substitutePlaceholderWithCommand(configPath, "POD_FQDN", "hostname -f")
		substitutePlaceholderWithCommand(configPath, "POD_SHORT_HOSTNAME", "hostname -s")
	}

	fmt.Fprintf(&command, "echo '%v' > %v; ", postprocessScript.String(), postprocessScriptPath)
	fmt.Fprintf(&command, "chmod +x '%v'; ", postprocessScriptPath)
	fmt.Fprintf(&command, "source %v; ", postprocessScriptPath)
	for _, configTemplatePath := range configTemplatePaths {
		fmt.Fprintf(&command, "cat %v; ", path.Join(consts.ConfigMountPoint, path.Base(configTemplatePath)))
	}

	return command.String()
}

func getDefaultEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: consts.ENV_K8S_POD_NAME,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: consts.ENV_K8S_POD_NAMESPACE,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: consts.ENV_K8S_NODE_NAME,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}

func getNativeClientConfigEnv() []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  consts.ClientConfigPathEnv,
		Value: path.Join(consts.ConfigTemplateMountPoint, consts.ClientConfigFileName),
	}}
}
