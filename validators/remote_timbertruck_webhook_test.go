package validators

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestValidateRemoteTimbertruck(t *testing.T) {
	t.Run("allows an ordinary remote instance", func(t *testing.T) {
		require.Empty(t, validateRemoteTimbertruck(&ytv1.CommonSpec{}, &ytv1.InstanceSpec{}))
	})

	t.Run("rejects common timbertruck settings", func(t *testing.T) {
		errors := validateRemoteTimbertruck(
			&ytv1.CommonSpec{Timbertruck: &ytv1.TimbertruckSpec{}},
			&ytv1.InstanceSpec{},
		)
		require.Len(t, errors, 1)
		require.Equal(t, "spec.timbertruck", errors[0].Field)
	})

	t.Run("rejects delivery flags including explicit false", func(t *testing.T) {
		errors := validateRemoteTimbertruck(
			&ytv1.CommonSpec{},
			&ytv1.InstanceSpec{StructuredLoggers: []ytv1.StructuredLoggerSpec{{
				Category:       ptr.To("Access"),
				EnableDelivery: ptr.To(false),
			}}},
		)
		require.Len(t, errors, 1)
		require.Equal(t, "spec.structuredLoggers[0].enableDelivery", errors[0].Field)
	})

	t.Run("rejects structured logger without categories", func(t *testing.T) {
		errors := validateRemoteTimbertruck(
			&ytv1.CommonSpec{},
			&ytv1.InstanceSpec{StructuredLoggers: []ytv1.StructuredLoggerSpec{{}}},
		)
		require.Len(t, errors, 1)
		require.Equal(t, "spec.structuredLoggers[0]", errors[0].Field)
	})
}
