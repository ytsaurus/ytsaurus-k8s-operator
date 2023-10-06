package components

import (
	"fmt"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
)

func createVolumeClaims(specVolumeClaimTemplates []ytv1.EmbeddedPersistentVolumeClaim) []v1.PersistentVolumeClaim {
	volumeClaims := make([]v1.PersistentVolumeClaim, 0, len(specVolumeClaimTemplates))
	for _, volumeClaim := range specVolumeClaimTemplates {
		volumeClaims = append(volumeClaims, v1.PersistentVolumeClaim{
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

func createConfigTemplateVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      consts.ConfigTemplateVolumeName,
		MountPath: consts.ConfigTemplateMountPoint,
		ReadOnly:  true,
	}
}

func createConfigVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      consts.ConfigVolumeName,
		MountPath: consts.ConfigMountPoint,
		ReadOnly:  false,
	}
}

func createVolumeMounts(specVolumeMounts []v1.VolumeMount) []v1.VolumeMount {
	volumeMounts := make([]v1.VolumeMount, 0, len(specVolumeMounts)+1)
	volumeMounts = append(volumeMounts, specVolumeMounts...)
	volumeMounts = append(volumeMounts, createConfigTemplateVolumeMount())
	volumeMounts = append(volumeMounts, createConfigVolumeMount())
	return volumeMounts
}

func createConfigVolume(volumeName string, configMapName string, mode *int32) v1.Volume {
	return v1.Volume{
		Name: volumeName,
		VolumeSource: v1.VolumeSource{
			ConfigMap: &v1.ConfigMapVolumeSource{
				LocalObjectReference: v1.LocalObjectReference{
					Name: configMapName,
				},
				DefaultMode: mode,
			},
		},
	}
}

func createConfigEmptyDir() v1.Volume {
	return v1.Volume{
		Name: consts.ConfigVolumeName,
		VolumeSource: v1.VolumeSource{
			EmptyDir: &v1.EmptyDirVolumeSource{},
		},
	}
}

func createServerVolumes(specVolumes []v1.Volume, configMapName string) []v1.Volume {
	volumes := make([]v1.Volume, 0, len(specVolumes)+1)
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
	postprocessScript += substituteEnvCommand("POD_NAME")
	postprocessScript += substituteEnvCommand("POD_NAMESPACE")

	command += fmt.Sprintf("echo '%v' > %v; ", postprocessScript, postprocessScriptPath)
	command += fmt.Sprintf("chmod +x '%v'; ", postprocessScriptPath)
	command += fmt.Sprintf("source %v; ", postprocessScriptPath)
	command += fmt.Sprintf("cat %v; ", configPath)

	return command
}

func getConfigPostprocessEnv() []v1.EnvVar {
	return []v1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}
}
