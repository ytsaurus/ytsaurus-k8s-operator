package components

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

var _ = Describe("FindVolumeMountForPath", func() {
	DescribeTable("returns the last volume mount covering the path",
		func(volumeMounts []corev1.VolumeMount, path string, wantName string) {
			got := FindVolumeMountForPath(volumeMounts, path)
			if wantName == "" {
				Expect(got).To(BeNil())
			} else {
				Expect(got).NotTo(BeNil())
				Expect(got.Name).To(Equal(wantName))
			}
		},
		Entry("path equals the mount path",
			[]corev1.VolumeMount{{Name: "data", MountPath: "/yt/data"}},
			"/yt/data", "data",
		),
		Entry("path is a sub-directory of the mount path",
			[]corev1.VolumeMount{{Name: "data", MountPath: "/yt/data"}},
			"/yt/data/snapshots", "data",
		),
		Entry("sibling path sharing a string prefix does not match",
			[]corev1.VolumeMount{{Name: "data", MountPath: "/yt/master-data"}},
			"/yt/master-data2", "",
		),
		Entry("no mount covers the path",
			[]corev1.VolumeMount{{Name: "data", MountPath: "/yt/data"}},
			"/yt/logs", "",
		),
		Entry("mount path with trailing slash does not match (no normalization)",
			[]corev1.VolumeMount{{Name: "data", MountPath: "/yt/data/"}},
			"/yt/data/snapshots", "",
		),
		Entry("last mount wins over an earlier mount of the same path",
			[]corev1.VolumeMount{
				{Name: "first", MountPath: "/yt/data"},
				{Name: "second", MountPath: "/yt/data"},
			},
			"/yt/data/snapshots", "second",
		),
		Entry("last mount wins even when an earlier mount is more specific",
			[]corev1.VolumeMount{
				{Name: "nested", MountPath: "/yt/data/snapshots"},
				{Name: "root", MountPath: "/yt"},
			},
			"/yt/data/snapshots", "root",
		),
		Entry("nested mount mounted later wins over its parent",
			[]corev1.VolumeMount{
				{Name: "root", MountPath: "/yt"},
				{Name: "nested", MountPath: "/yt/data"},
			},
			"/yt/data/snapshots", "nested",
		),
	)
})

var _ = Describe("resolveLocationMounts", func() {
	type wantMount struct {
		Name        string
		MountPath   string
		SubPath     string
		SubPathExpr string
	}

	DescribeTable("resolves locations to granular sub-path mounts",
		func(
			volumeMounts []corev1.VolumeMount,
			locations []ytv1.LocationSpec,
			requiredLocations []ytv1.LocationType,
			want []wantMount,
		) {
			instanceSpec := &ytv1.InstanceSpec{
				VolumeMounts: volumeMounts,
				Locations:    locations,
			}

			got, err := resolveLocationMounts(instanceSpec, requiredLocations...)
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(HaveLen(len(want)))

			for i, w := range want {
				Expect(got[i].Name).To(Equal(w.Name), "mount[%d].Name", i)
				Expect(got[i].MountPath).To(Equal(w.MountPath), "mount[%d].MountPath", i)
				Expect(got[i].SubPath).To(Equal(w.SubPath), "mount[%d].SubPath", i)
				Expect(got[i].SubPathExpr).To(Equal(w.SubPathExpr), "mount[%d].SubPathExpr", i)
				Expect(got[i].ReadOnly).To(BeFalseBecause("mount[%d] must not set ReadOnly", i))
			}
		},
		Entry("location is a sub-directory of the volume",
			[]corev1.VolumeMount{{Name: "master-data", MountPath: "/yt/master-data"}},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/snapshots"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots},
			[]wantMount{
				{Name: "master-data", MountPath: "/yt/master-data/snapshots", SubPath: "snapshots"},
			},
		),
		Entry("location equals the volume mount path (volume root)",
			[]corev1.VolumeMount{{Name: "logs", MountPath: "/yt/logs"}},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeLogs, Path: "/yt/logs"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeLogs},
			[]wantMount{
				{Name: "logs", MountPath: "/yt/logs", SubPath: ""},
			},
		),
		Entry("trailing slash on location path",
			[]corev1.VolumeMount{{Name: "master-data", MountPath: "/yt/master-data"}},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/snapshots/"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots},
			[]wantMount{
				{Name: "master-data", MountPath: "/yt/master-data/snapshots/", SubPath: "snapshots"},
			},
		),
		Entry("volume mount already has a sub-path",
			[]corev1.VolumeMount{
				{Name: "storage", MountPath: "/yt/master-data", SubPath: "cluster-a"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/snapshots"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots},
			[]wantMount{
				{Name: "storage", MountPath: "/yt/master-data/snapshots", SubPath: "cluster-a/snapshots"},
			},
		),
		Entry("volume mount has a sub-path and location is the volume root",
			[]corev1.VolumeMount{
				{Name: "storage", MountPath: "/yt/master-data", SubPath: "cluster-a"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots},
			[]wantMount{
				{Name: "storage", MountPath: "/yt/master-data", SubPath: "cluster-a"},
			},
		),
		Entry("multiple locations on different volumes preserve order",
			[]corev1.VolumeMount{
				{Name: "snapshots-vol", MountPath: "/yt/snapshots"},
				{Name: "changelogs-vol", MountPath: "/yt/changelogs"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/snapshots/data"},
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/changelogs/data"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots, ytv1.LocationTypeMasterChangelogs},
			[]wantMount{
				{Name: "snapshots-vol", MountPath: "/yt/snapshots/data", SubPath: "data"},
				{Name: "changelogs-vol", MountPath: "/yt/changelogs/data", SubPath: "data"},
			},
		),
		Entry("volume mount with subPathExpr keeps the expression",
			[]corev1.VolumeMount{
				{Name: "storage", MountPath: "/yt/master-data", SubPathExpr: "$(K8S_POD_NAME)"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/snapshots"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots},
			[]wantMount{
				{Name: "storage", MountPath: "/yt/master-data/snapshots", SubPathExpr: "$(K8S_POD_NAME)/snapshots"},
			},
		),
		Entry("parent location is mounted before a nested one",
			[]corev1.VolumeMount{
				{Name: "vol-a", MountPath: "/yt/data"},
				{Name: "vol-b", MountPath: "/yt/data/snapshots"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/data/snapshots"},
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/data"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots, ytv1.LocationTypeMasterChangelogs},
			[]wantMount{
				{Name: "vol-a", MountPath: "/yt/data", SubPath: ""},
				{Name: "vol-b", MountPath: "/yt/data/snapshots", SubPath: ""},
			},
		),
		Entry("locations sharing one path produce a single mount",
			[]corev1.VolumeMount{
				{Name: "master-data", MountPath: "/yt/master-data"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/store"},
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/master-data/store"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots, ytv1.LocationTypeMasterChangelogs},
			[]wantMount{
				{Name: "master-data", MountPath: "/yt/master-data/store", SubPath: "store"},
			},
		),
		Entry("two locations on the same volume are mounted separately (no de-dup)",
			[]corev1.VolumeMount{
				{Name: "master-data", MountPath: "/yt/master-data"},
			},
			[]ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeMasterSnapshots, Path: "/yt/master-data/snapshots"},
				{LocationType: ytv1.LocationTypeMasterChangelogs, Path: "/yt/master-data/changelogs"},
			},
			[]ytv1.LocationType{ytv1.LocationTypeMasterSnapshots, ytv1.LocationTypeMasterChangelogs},
			[]wantMount{
				{Name: "master-data", MountPath: "/yt/master-data/snapshots", SubPath: "snapshots"},
				{Name: "master-data", MountPath: "/yt/master-data/changelogs", SubPath: "changelogs"},
			},
		),
	)

	It("errors when a required location is missing", func() {
		instanceSpec := &ytv1.InstanceSpec{
			VolumeMounts: []corev1.VolumeMount{{Name: "master-data", MountPath: "/yt/master-data"}},
			Locations:    []ytv1.LocationSpec{},
		}
		_, err := resolveLocationMounts(instanceSpec, ytv1.LocationTypeMasterSnapshots)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("MasterSnapshots"))
	})

	It("errors when a location is not covered by any volume mount", func() {
		instanceSpec := &ytv1.InstanceSpec{
			VolumeMounts: []corev1.VolumeMount{{Name: "other", MountPath: "/yt/other"}},
			Locations: []ytv1.LocationSpec{
				{LocationType: ytv1.LocationTypeLogs, Path: "/yt/logs/data"},
			},
		}
		_, err := resolveLocationMounts(instanceSpec, ytv1.LocationTypeLogs)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no volume mount covers"))
	})
})
