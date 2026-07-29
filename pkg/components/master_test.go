package components

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

var _ = Describe("addHydraPersistenceUploaderToPodSpec", func() {
	type wantDataMount struct {
		Name      string
		MountPath string
		SubPath   string
	}

	DescribeTable("derives sidecar data mounts from the instance spec",
		func(
			specVolumeMounts []corev1.VolumeMount,
			locations []ytv1.LocationSpec,
			wantDataMounts []wantDataMount,
		) {
			instanceSpec := &ytv1.InstanceSpec{
				VolumeMounts: specVolumeMounts,
				Locations:    locations,
			}

			mainContainerMounts := createVolumeMounts(specVolumeMounts)
			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:         "ytserver",
						VolumeMounts: mainContainerMounts,
					},
				},
			}

			err := addHydraPersistenceUploaderToPodSpec(
				"ghcr.io/ytsaurus/sidecars:0.0.1",
				podSpec,
				"http-proxies.ytsaurus-dev.svc",
				"robot-hydra-persistence-uploader-secret",
				instanceSpec,
			)
			Expect(err).NotTo(HaveOccurred())

			var sidecar *corev1.Container
			for i := range podSpec.Containers {
				if podSpec.Containers[i].Name == consts.HydraPersistenceUploaderContainerName {
					sidecar = &podSpec.Containers[i]
					break
				}
			}
			Expect(sidecar).NotTo(BeNil(), "hydra-persistence-uploader sidecar must be added")

			Expect(sidecar.VolumeMounts).To(HaveLen(1 + len(wantDataMounts) + 1))

			gotDataMounts := sidecar.VolumeMounts[1 : 1+len(wantDataMounts)]
			for i, want := range wantDataMounts {
				got := gotDataMounts[i]
				Expect(got.Name).To(Equal(want.Name))
				Expect(got.MountPath).To(Equal(want.MountPath))
				Expect(got.SubPath).To(Equal(want.SubPath))
				Expect(got.ReadOnly).To(BeTrueBecause("data mount %q must be read-only", want.MountPath))
			}

			var hasSharedBinariesVolume bool
			for _, v := range podSpec.Volumes {
				if v.Name == "shared-binaries" {
					hasSharedBinariesVolume = true
					Expect(v.EmptyDir).NotTo(BeNil(), "shared-binaries must be backed by emptyDir")
					break
				}
			}
			Expect(hasSharedBinariesVolume).To(BeTrueBecause("a shared-binaries volume must be added to the pod"))

			var ytserver *corev1.Container
			for i := range podSpec.Containers {
				if podSpec.Containers[i].Name == "ytserver" {
					ytserver = &podSpec.Containers[i]
					break
				}
			}
			Expect(ytserver).NotTo(BeNil())
			Expect(ytserver.Lifecycle).NotTo(BeNil())
			Expect(ytserver.Lifecycle.PostStart).NotTo(BeNil())
			var hasShared bool
			for _, vm := range ytserver.VolumeMounts {
				if vm.Name == "shared-binaries" {
					hasShared = true
					break
				}
			}
			Expect(hasShared).To(BeTrueBecause("ytserver must mount the shared-binaries volume"))
		},
		Entry("default spec - master-data at /yt/master-data",
			[]corev1.VolumeMount{
				{Name: "master-data", MountPath: "/yt/master-data"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/master-data/master-changelogs"},
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/master-snapshots"},
			},
			[]wantDataMount{
				{Name: "master-data", MountPath: "/yt/master-data/master-snapshots", SubPath: "master-snapshots"},
				{Name: "master-data", MountPath: "/yt/master-data/master-changelogs", SubPath: "master-changelogs"},
			},
		),
		Entry("custom volume name and path",
			[]corev1.VolumeMount{
				{Name: "storage", MountPath: "/data/master"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/data/master/changelogs"},
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/data/master/snapshots"},
			},
			[]wantDataMount{
				{Name: "storage", MountPath: "/data/master/snapshots", SubPath: "snapshots"},
				{Name: "storage", MountPath: "/data/master/changelogs", SubPath: "changelogs"},
			},
		),
		Entry("snapshots and changelogs on different volumes",
			[]corev1.VolumeMount{
				{Name: "snapshots-vol", MountPath: "/yt/snapshots"},
				{Name: "changelogs-vol", MountPath: "/yt/changelogs"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/snapshots/data"},
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/changelogs/data"},
			},
			[]wantDataMount{
				{Name: "snapshots-vol", MountPath: "/yt/snapshots/data", SubPath: "data"},
				{Name: "changelogs-vol", MountPath: "/yt/changelogs/data", SubPath: "data"},
			},
		),
	)

	It("errors and adds no sidecar when a required location is missing", func() {
		instanceSpec := &ytv1.InstanceSpec{
			VolumeMounts: []corev1.VolumeMount{
				{Name: "master-data", MountPath: "/yt/master-data"},
			},
			Locations: []ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/master-data/master-changelogs"},
			},
		}

		podSpec := &corev1.PodSpec{
			Containers: []corev1.Container{{Name: "ytserver"}},
		}

		err := addHydraPersistenceUploaderToPodSpec(
			"img", podSpec, "proxy", "secret", instanceSpec,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("MasterSnapshots"))

		for _, c := range podSpec.Containers {
			Expect(c.Name).NotTo(Equal(consts.HydraPersistenceUploaderContainerName))
		}
	})
})
