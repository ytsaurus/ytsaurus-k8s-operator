package components

import (
	"fmt"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
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

func createServerVolumes(specVolumes []corev1.Volume, configMapName string) []corev1.Volume {
	volumes := make([]corev1.Volume, 0, len(specVolumes)+1)
	volumes = append(volumes, specVolumes...)

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

	postprocessScript := fmt.Sprintf("cp %v %v; ", configTemplatePath, configPath)

	for _, envVar := range getConfigPostprocessEnv() {
		postprocessScript += substituteEnvCommand(envVar.Name)
	}

	command += fmt.Sprintf("echo '%v' > %v; ", postprocessScript, postprocessScriptPath)
	command += fmt.Sprintf("chmod +x '%v'; ", postprocessScriptPath)
	command += fmt.Sprintf("source %v; ", postprocessScriptPath)
	command += fmt.Sprintf("cat %v; ", configPath)

	return command
}

func getConfigPostprocessEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: "K8S_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "K8S_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "K8S_NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
	}
}
