package components

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/yson"
	"sigs.k8s.io/yaml"
)

func TestGetTimbertruckConfig(t *testing.T) {
	timbertruckConfig := ytconfig.NewTimbertruckConfig(
		[]v1.StructuredLoggerSpec{
			{
				Category: "Access",
				BaseLoggerSpec: v1.BaseLoggerSpec{
					Name:               "access",
					Format:             v1.LogFormatJson,
					Compression:        v1.LogCompressionNone,
					UseTimestampSuffix: true,
				},
			},
			{
				Category: "Security",
				BaseLoggerSpec: v1.BaseLoggerSpec{
					Name:        "security",
					Format:      v1.LogFormatYson,
					Compression: v1.LogCompressionZstd,
				},
			},
			{
				Category: "Other",
				BaseLoggerSpec: v1.BaseLoggerSpec{
					Name:        "other",
					Format:      v1.LogFormatPlainText,
					Compression: v1.LogCompressionZstd,
				},
			},
		},
		"/yt/master-logs/timbertruck",
		"master",
		"/yt/master-logs",
		"http-proxies-lb.ytsaurus-dev.svc.cluster.local",
		"//sys/admin/logs",
	)

	ysonData, err := timbertruckConfig.ToYSON()
	require.NoError(t, err)

	var config any
	require.NoError(t, yson.Unmarshal(ysonData, &config))
	yamlData, err := yaml.Marshal(config)
	require.NoError(t, err)

	canonize.Assert(t, []byte(strings.TrimSpace(string(yamlData))))
}

var _ = Describe("buildTimbertruckVolumeMounts", func() {
	const configVolumeName = consts.TimbertruckContainerName + "-config"

	It("derives the log mount from the instance spec and mounts the config read-only", func() {
		instanceSpec := &v1.InstanceSpec{
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data-vol", MountPath: "/yt/data"},
				{Name: "logs-vol", MountPath: "/yt/logs"},
			},
			Locations: []v1.LocationSpec{
				{LocationType: v1.LocationTypeLogs, Path: "/yt/logs/master-logs/logs"},
			},
		}

		mounts, err := buildTimbertruckVolumeMounts(instanceSpec, configVolumeName)
		Expect(err).NotTo(HaveOccurred())
		Expect(mounts).To(HaveLen(2))

		Expect(mounts[0].Name).To(Equal("logs-vol"))
		Expect(mounts[0].MountPath).To(Equal("/yt/logs/master-logs/logs"))
		Expect(mounts[0].SubPath).To(Equal("master-logs/logs"))
		Expect(mounts[0].ReadOnly).To(BeFalseBecause("timbertruck must be able to write to logs location"))

		Expect(mounts[1].Name).To(Equal(configVolumeName))
		Expect(mounts[1].MountPath).To(Equal("/etc/timbertruck"))
		Expect(mounts[1].ReadOnly).To(BeTrueBecause("timbertruck config must be mounted read-only"))
	})

	It("errors when no logs location is defined", func() {
		instanceSpec := &v1.InstanceSpec{
			VolumeMounts: []corev1.VolumeMount{
				{Name: "logs-vol", MountPath: "/yt/logs"},
			},
		}
		_, err := buildTimbertruckVolumeMounts(instanceSpec, configVolumeName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to resolve mounts"))
	})

	It("errors when the logs location is not covered by any volume mount", func() {
		instanceSpec := &v1.InstanceSpec{
			VolumeMounts: []corev1.VolumeMount{
				{Name: "logs-vol", MountPath: "/yt/logs"},
			},
			Locations: []v1.LocationSpec{
				{LocationType: v1.LocationTypeLogs, Path: "/yt/logs-wrong/master-logs/logs"},
			},
		}
		_, err := buildTimbertruckVolumeMounts(instanceSpec, configVolumeName)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to resolve mounts"))
	})
})
