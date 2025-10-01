package components

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

func TestGetTimbertruckSupervisorScript(t *testing.T) {
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
	timbertruckSupervisorScript, err := getTimbertruckSupervisorScript(timbertruckConfig)
	require.NoError(t, err)
	canonize.Assert(t, []byte(timbertruckSupervisorScript))
}
