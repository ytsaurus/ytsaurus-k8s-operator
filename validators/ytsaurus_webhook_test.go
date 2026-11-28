package validators

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestValidateInstanceLoggers(t *testing.T) {
	policy := &ytv1.LogRotationPolicy{
		MaxTotalSizeToKeep: ptr.To(resource.MustParse("60Mi")),
		MaxSegmentSize:     ptr.To(resource.MustParse("10Mi")),
	}
	fileLogger := ytv1.TextLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:           "info",
			MinLogLevel:    ytv1.LogLevelInfo,
			RotationPolicy: policy,
		},
		WriterType: ytv1.LogWriterTypeFile,
	}
	stderrLogger := ytv1.TextLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:        "stderr",
			MinLogLevel: ytv1.LogLevelWarning,
		},
		WriterType: ytv1.LogWriterTypeStderr,
	}
	structuredLogger := ytv1.StructuredLoggerSpec{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{
			Name:           "access",
			RotationPolicy: policy,
		},
		Category: ptr.To("Access"),
	}
	unboundedStructuredLogger := *structuredLogger.DeepCopy()
	unboundedStructuredLogger.RotationPolicy = nil
	unboundedTextLogger := *fileLogger.DeepCopy()
	unboundedTextLogger.RotationPolicy = nil
	timestampStructuredLogger := *structuredLogger.DeepCopy()
	timestampStructuredLogger.UseTimestampSuffix = true
	conflictingStructuredLogger := *timestampStructuredLogger.DeepCopy()
	conflictingStructuredLogger.UseDateTimeSuffix = true
	stderrWithSuffixes := *stderrLogger.DeepCopy()
	stderrWithSuffixes.UseTimestampSuffix = true
	stderrWithSuffixes.UseDateTimeSuffix = true
	stderrWithRotation := *stderrWithSuffixes.DeepCopy()
	stderrWithRotation.RotationPolicy = policy

	for _, testCase := range []struct {
		name         string
		loggers      []ytv1.TextLoggerSpec
		structured   []ytv1.StructuredLoggerSpec
		logsQuota    string
		wantErrors   []string
		wantWarnings []string
	}{
		{
			name:         "default text loggers warn",
			wantWarnings: []string{"spec.discovery"},
		},
		{
			name:         "bounded structured logger still uses default text loggers",
			structured:   []ytv1.StructuredLoggerSpec{structuredLogger},
			wantWarnings: []string{"spec.discovery"},
		},
		{
			name:       "bounded text and structured loggers",
			loggers:    []ytv1.TextLoggerSpec{fileLogger},
			structured: []ytv1.StructuredLoggerSpec{structuredLogger},
		},
		{
			name:       "bounded loggers below logs quota",
			loggers:    []ytv1.TextLoggerSpec{fileLogger},
			structured: []ytv1.StructuredLoggerSpec{structuredLogger},
			logsQuota:  "1Gi",
		},
		{
			name:       "bounded loggers equal logs quota including active segments",
			loggers:    []ytv1.TextLoggerSpec{fileLogger},
			structured: []ytv1.StructuredLoggerSpec{structuredLogger},
			logsQuota:  "140Mi",
		},
		{
			name:         "bounded loggers exceed logs quota including active segments",
			loggers:      []ytv1.TextLoggerSpec{fileLogger},
			structured:   []ytv1.StructuredLoggerSpec{structuredLogger},
			logsQuota:    "139Mi",
			wantWarnings: []string{"spec.discovery"},
		},
		{
			name:         "unrotated text logger",
			loggers:      []ytv1.TextLoggerSpec{unboundedTextLogger},
			structured:   []ytv1.StructuredLoggerSpec{structuredLogger},
			wantWarnings: []string{"spec.discovery.loggers[0].rotationPolicy", "spec.discovery"},
		},
		{
			name:         "unrotated structured logger",
			loggers:      []ytv1.TextLoggerSpec{fileLogger},
			structured:   []ytv1.StructuredLoggerSpec{unboundedStructuredLogger},
			wantWarnings: []string{"spec.discovery.structuredLoggers[0].rotationPolicy", "spec.discovery"},
		},
		{
			name:       "both unrotated loggers warn",
			loggers:    []ytv1.TextLoggerSpec{unboundedTextLogger},
			structured: []ytv1.StructuredLoggerSpec{unboundedStructuredLogger},
			wantWarnings: []string{
				"spec.discovery.loggers[0].rotationPolicy",
				"spec.discovery.structuredLoggers[0].rotationPolicy",
				"spec.discovery",
			},
		},
		{
			name:         "structured timestamp suffix warns",
			loggers:      []ytv1.TextLoggerSpec{stderrLogger},
			structured:   []ytv1.StructuredLoggerSpec{timestampStructuredLogger},
			wantWarnings: []string{"spec.discovery.structuredLoggers[0].useTimestampSuffix"},
		},
		{
			name:       "structured suffix conflict is rejected",
			loggers:    []ytv1.TextLoggerSpec{stderrLogger},
			structured: []ytv1.StructuredLoggerSpec{conflictingStructuredLogger},
			wantErrors: []string{"spec.discovery.structuredLoggers[0].useTimestampSuffix"},
		},
		{
			name:    "stderr ignores file suffixes",
			loggers: []ytv1.TextLoggerSpec{stderrWithSuffixes},
		},
		{
			name:         "stderr still warns about rotation policy",
			loggers:      []ytv1.TextLoggerSpec{stderrWithRotation},
			wantWarnings: []string{"spec.discovery.loggers[0].rotationPolicy"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			spec := ytv1.InstanceSpec{
				Loggers:           testCase.loggers,
				StructuredLoggers: testCase.structured,
			}
			if testCase.logsQuota != "" {
				spec.VolumeMounts = []corev1.VolumeMount{{Name: "logs", MountPath: "/var/log"}}
				spec.Locations = []ytv1.LocationSpec{{
					LocationType: ytv1.LocationTypeLogs,
					Path:         "/var/log",
					Quota:        ptr.To(resource.MustParse(testCase.logsQuota)),
				}}
			}
			validator := baseValidator{}
			results := validator.validateInstanceSpec(spec, &ytv1.CommonSpec{}, field.NewPath("spec", "discovery"))
			require.Len(t, results.Errors, len(testCase.wantErrors))
			for i, path := range testCase.wantErrors {
				require.Equal(t, path, results.Errors[i].Field)
			}
			require.Len(t, results.Warnings, len(testCase.wantWarnings))
			for i, path := range testCase.wantWarnings {
				var warning *field.Error
				require.ErrorAs(t, results.Warnings[i], &warning)
				require.Equal(t, path, warning.Field)
			}
		})
	}
}

func TestValidateDedicatedMasterUpdatePlan(t *testing.T) {
	primary := ytv1.ComponentUpdateSelector{Component: ytv1.Component{Type: ytv1.MasterType}}
	secondary := ytv1.ComponentUpdateSelector{
		Component: ytv1.Component{Type: ytv1.MasterType, Name: "12"},
		Strategy:  &ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}},
	}
	proxy := ytv1.ComponentUpdateSelector{Component: ytv1.Component{Type: ytv1.HttpProxyType}}
	yql := ytv1.ComponentUpdateSelector{Component: ytv1.Component{Type: ytv1.YqlAgentType}}
	for _, tc := range []struct {
		name        string
		plan        []ytv1.ComponentUpdateSelector
		wantError   string
		maintenance bool
	}{
		{"Everything", []ytv1.ComponentUpdateSelector{{Class: ytv1.ComponentClassEverything}}, "", false},
		{"coordinated master-cell maintenance", []ytv1.ComponentUpdateSelector{primary, secondary}, "", true},
		{"primary cell", []ytv1.ComponentUpdateSelector{primary}, "", false},
		{"secondary cell", []ytv1.ComponentUpdateSelector{secondary}, "", false},
		{"unknown secondary", []ytv1.ComponentUpdateSelector{{Component: ytv1.Component{Type: ytv1.MasterType, Name: "13"}}}, "configured secondary master cell tag", false},
		{"two cells", []ytv1.ComponentUpdateSelector{primary, secondary}, "exactly one selector", false},
		{"primary with proxy", []ytv1.ComponentUpdateSelector{primary, proxy}, "exactly one selector", false},
		{"proxy with secondary", []ytv1.ComponentUpdateSelector{proxy, secondary}, "exactly one selector", false},
		{"non-master components", []ytv1.ComponentUpdateSelector{proxy, yql}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &ytv1.Ytsaurus{Spec: ytv1.YtsaurusSpec{
				PrimaryMasters:   ytv1.MastersSpec{MasterConnectionSpec: ytv1.MasterConnectionSpec{CellTag: 10}},
				SecondaryMasters: []ytv1.MastersSpec{{MasterConnectionSpec: ytv1.MasterConnectionSpec{CellTag: 12}}},
				UpdatePlan:       tc.plan,
			}}
			if tc.maintenance {
				cluster.Spec.ClusterMaintenance = &ytv1.ClusterMaintenance{Shutdown: ytv1.ClusterShutdownExceptMasters}
			}
			validator := baseValidator{}
			err := validator.validateUpdatePlan(cluster).Errors.ToAggregate()
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
		})
	}
}
