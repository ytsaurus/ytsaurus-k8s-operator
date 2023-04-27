package components

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createConfigVolumeMount() v1.VolumeMount {
	return v1.VolumeMount{
		Name:      consts.ConfigVolumeName,
		MountPath: consts.ConfigMountPoint,
		ReadOnly:  true,
	}
}

func createVolumeMounts(specVolumeMounts []v1.VolumeMount) []v1.VolumeMount {
	volumeMounts := make([]v1.VolumeMount, 0, len(specVolumeMounts)+1)
	volumeMounts = append(volumeMounts, specVolumeMounts...)
	volumeMounts = append(volumeMounts, createConfigVolumeMount())
	return volumeMounts
}

func createConfigVolume(configMapName string, mode *int32) v1.Volume {
	return v1.Volume{
		Name: consts.ConfigVolumeName,
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

func createVolumes(specVolumes []v1.Volume, configMapName string) []v1.Volume {
	volumes := make([]v1.Volume, 0, len(specVolumes)+1)
	volumes = append(volumes, specVolumes...)

	volumes = append(volumes, createConfigVolume(configMapName, nil))
	return volumes
}

func getLocationInitCommand(locations []ytv1.LocationSpec) string {
	locationInitCommand := "echo 'Init locations'"
	for _, location := range locations {
		locationInitCommand += "; mkdir -p " + location.Path
	}
	return locationInitCommand
}
