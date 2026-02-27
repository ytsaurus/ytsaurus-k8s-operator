package components

import (
	"fmt"
	"path"

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
	command := "echo 'Init locations'; "
	for _, location := range locations {
		command += "mkdir -p " + location.Path + "; "
	}
	return command
}

func getConfigPostprocessingCommand(configFileName string) string {
	command := fmt.Sprintf("echo 'Postprocess config %v';", configFileName)

	// Store postprocessing as a script on filesystem to ease up manual
	// config re-initialization without pod recreation. This will be useful
	// when operator starts restarting processes without container recreation.

	configTemplatePath := path.Join(consts.ConfigTemplateMountPoint, configFileName)
	configPath := path.Join(consts.ConfigMountPoint, configFileName)
	postprocessScriptPath := path.Join(consts.ConfigMountPoint, consts.PostprocessConfigScriptFileName)

	substituteEnvCommand := func(envVar string) string {
		// Replace placeholder {envVar} with the actual value of environment variable envVar.
		return fmt.Sprintf("sed -i -s \"s/{%v}/${%v}/g\" %v; ", envVar, envVar, configPath)
	}

	substitutePlaceholderWithCommand := func(placeholder string, command string) string {
		// Replace placeholder {placeholder} with the output of the given command.
		return fmt.Sprintf("sed -i -s \"s/{%v}/$(%v)/g\" %v; ", placeholder, command, configPath)
	}

	postprocessScript := fmt.Sprintf("cp %v %v; ", configTemplatePath, configPath)

	for _, envVar := range getDefaultEnv() {
		postprocessScript += substituteEnvCommand(envVar.Name)
	}

	postprocessScript += substitutePlaceholderWithCommand("POD_FQDN", "hostname -f")
	postprocessScript += substitutePlaceholderWithCommand("POD_SHORT_HOSTNAME", "hostname -s")

	command += fmt.Sprintf("echo '%v' > %v; ", postprocessScript, postprocessScriptPath)
	command += fmt.Sprintf("chmod +x '%v'; ", postprocessScriptPath)
	command += fmt.Sprintf("source %v; ", postprocessScriptPath)
	command += fmt.Sprintf("cat %v; ", configPath)

	return command
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
