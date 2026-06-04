package components

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

func structuredLogger(name string, enableDelivery *bool) ytv1.StructuredLoggerSpec {
	return ytv1.StructuredLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{Name: name, Format: ytv1.LogFormatJson},
		EnableDelivery: enableDelivery,
	}
}

func loggerNames(loggers []ytv1.StructuredLoggerSpec) []string {
	names := make([]string, 0, len(loggers))
	for _, l := range loggers {
		names = append(names, l.Name)
	}
	return names
}

func TestDeliveredStructuredLoggers(t *testing.T) {
	tt := &ytv1.TimbertruckSpec{Image: ptr.To("img")}

	t.Run("legacy master delivers all when no per-log flags", func(t *testing.T) {
		loggers := []ytv1.StructuredLoggerSpec{structuredLogger("access", nil), structuredLogger("security", nil)}
		require.Equal(t, []string{"access", "security"}, loggerNames(deliveredStructuredLoggers(tt, loggers)))
	})

	t.Run("no component spec and no per-log flags delivers nothing", func(t *testing.T) {
		loggers := []ytv1.StructuredLoggerSpec{structuredLogger("access", nil)}
		require.Empty(t, deliveredStructuredLoggers(nil, loggers))
	})

	t.Run("per-log flags select only enabled loggers", func(t *testing.T) {
		loggers := []ytv1.StructuredLoggerSpec{
			structuredLogger("access", ptr.To(true)),
			structuredLogger("security", ptr.To(false)),
			structuredLogger("other", nil),
		}
		require.Equal(t, []string{"access"}, loggerNames(deliveredStructuredLoggers(nil, loggers)))
	})

	t.Run("per-log flags take precedence over legacy master mode", func(t *testing.T) {
		loggers := []ytv1.StructuredLoggerSpec{
			structuredLogger("access", ptr.To(true)),
			structuredLogger("security", nil),
		}
		require.Equal(t, []string{"access"}, loggerNames(deliveredStructuredLoggers(tt, loggers)))
	})
}

func TestEffectiveTimbertruckImage(t *testing.T) {
	component := &ytv1.TimbertruckSpec{Image: ptr.To("component-img")}
	common := &ytv1.TimbertruckSpec{Image: ptr.To("common-img")}

	require.Equal(t, "component-img", effectiveTimbertruckImage(component, common))
	require.Equal(t, "common-img", effectiveTimbertruckImage(nil, common))
	require.Equal(t, "common-img", effectiveTimbertruckImage(&ytv1.TimbertruckSpec{}, common))
	require.Equal(t, "", effectiveTimbertruckImage(nil, nil))
}

func TestEffectiveLogsDeliveryPath(t *testing.T) {
	require.Equal(t, "//component/path", effectiveLogsDeliveryPath(
		&ytv1.TimbertruckSpec{DirectoryPath: ptr.To("//component/path")},
		&ytv1.TimbertruckSpec{DirectoryPath: ptr.To("//common/path")},
	))
	require.Equal(t, "//common/path", effectiveLogsDeliveryPath(
		nil,
		&ytv1.TimbertruckSpec{DirectoryPath: ptr.To("//common/path")},
	))
	require.Equal(t, consts.DefaultTimbertruckDirectoryPath, effectiveLogsDeliveryPath(nil, nil))
}

func instanceSpec(loggers []ytv1.StructuredLoggerSpec, withLogsLocation bool) *ytv1.InstanceSpec {
	spec := &ytv1.InstanceSpec{StructuredLoggers: loggers}
	if withLogsLocation {
		spec.Locations = []ytv1.LocationSpec{{LocationType: ytv1.LocationTypeLogs, Path: "/yt/logs"}}
	}
	return spec
}

func TestResolveTimbertruckDelivery(t *testing.T) {
	loggers := []ytv1.StructuredLoggerSpec{structuredLogger("access", ptr.To(true))}

	t.Run("disabled without image", func(t *testing.T) {
		require.Nil(t, resolveTimbertruckDelivery(nil, nil, instanceSpec(loggers, true)))
	})

	t.Run("disabled without logs location", func(t *testing.T) {
		require.Nil(t, resolveTimbertruckDelivery(nil, &ytv1.TimbertruckSpec{Image: ptr.To("common-img")},
			instanceSpec(loggers, false)))
	})

	t.Run("enabled via cluster-wide image", func(t *testing.T) {
		delivery := resolveTimbertruckDelivery(nil, &ytv1.TimbertruckSpec{Image: ptr.To("common-img")}, instanceSpec(loggers, true))
		require.NotNil(t, delivery)
		require.Equal(t, "common-img", delivery.Image)
		require.Equal(t, "/yt/logs", delivery.LogsDirectory)
		require.Equal(t, consts.DefaultTimbertruckDirectoryPath, delivery.LogsDeliveryPath)
		require.Equal(t, []string{"access"}, loggerNames(delivery.Loggers))
	})

	t.Run("master legacy with component image", func(t *testing.T) {
		component := &ytv1.TimbertruckSpec{Image: ptr.To("master-img"), DirectoryPath: ptr.To("//custom")}
		delivery := resolveTimbertruckDelivery(component, nil, instanceSpec([]ytv1.StructuredLoggerSpec{structuredLogger("access", nil)}, true))
		require.NotNil(t, delivery)
		require.Equal(t, "master-img", delivery.Image)
		require.Equal(t, "//custom", delivery.LogsDeliveryPath)
	})

	t.Run("disabled when no loggers to deliver", func(t *testing.T) {
		require.Nil(t, resolveTimbertruckDelivery(nil, &ytv1.TimbertruckSpec{Image: ptr.To("img")},
			instanceSpec([]ytv1.StructuredLoggerSpec{structuredLogger("access", nil)}, true)))
	})
}
