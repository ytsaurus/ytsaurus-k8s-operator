package validators

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

func guaranteedTestResources(cpu string) corev1.ResourceRequirements {
	limits := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(cpu),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}
	return corev1.ResourceRequirements{Limits: limits, Requests: limits.DeepCopy()}
}

func containerTestSpec(t *testing.T, name string, resources corev1.ResourceRequirements) string {
	t.Helper()
	data, err := yaml.Marshal(corev1.Container{Name: name, Image: "test", Resources: resources})
	require.NoError(t, err)
	return string(data)
}

func stderrTestLoggers() []ytv1.TextLoggerSpec {
	return []ytv1.TextLoggerSpec{{
		BaseLoggerSpec: ytv1.BaseLoggerSpec{Name: "stderr", MinLogLevel: ytv1.LogLevelWarning},
		WriterType:     ytv1.LogWriterTypeStderr,
	}}
}

func TestHasGuaranteedResources(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		modify func(*corev1.ResourceRequirements)
		want   bool
	}{
		{name: "equal requests and limits", want: true},
		{name: "limits only", modify: func(r *corev1.ResourceRequirements) { r.Requests = nil }, want: true},
		{name: "omitted memory request", modify: func(r *corev1.ResourceRequirements) { delete(r.Requests, corev1.ResourceMemory) }, want: true},
		{name: "equivalent CPU quantities", modify: func(r *corev1.ResourceRequirements) {
			r.Requests[corev1.ResourceCPU] = resource.MustParse("2000m")
		}, want: true},
		{name: "zero CPU request", modify: func(r *corev1.ResourceRequirements) {
			r.Requests[corev1.ResourceCPU] = resource.MustParse("0")
		}},
		{name: "unequal memory request", modify: func(r *corev1.ResourceRequirements) {
			r.Requests[corev1.ResourceMemory] = resource.MustParse("1Gi")
		}},
		{name: "missing CPU limit", modify: func(r *corev1.ResourceRequirements) { delete(r.Limits, corev1.ResourceCPU) }},
		{name: "missing memory limit", modify: func(r *corev1.ResourceRequirements) { delete(r.Limits, corev1.ResourceMemory) }},
		{name: "zero limits", modify: func(r *corev1.ResourceRequirements) {
			r.Limits[corev1.ResourceCPU] = resource.MustParse("0")
			r.Requests = nil
		}},
		{name: "no resources", modify: func(r *corev1.ResourceRequirements) { *r = corev1.ResourceRequirements{} }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resources := guaranteedTestResources("2")
			if testCase.modify != nil {
				testCase.modify(&resources)
			}
			original := resources.DeepCopy()
			require.Equal(t, testCase.want, hasGuaranteedResources(resources))
			require.Equal(t, *original, resources)
		})
	}
}

func TestValidateExecNodeQoS(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		modify       func(*ytv1.ExecNodesSpec)
		wantErrors   []string
		wantWarnings []string
	}{
		{name: "Guaranteed containers and fractional init CPU"},
		{name: "main limits only", modify: func(s *ytv1.ExecNodesSpec) { s.Resources.Requests = nil }},
		{name: "job limits only", modify: func(s *ytv1.ExecNodesSpec) { s.JobResources.Requests = nil }},
		{name: "omitted isolated job resources", modify: func(s *ytv1.ExecNodesSpec) { s.JobResources = nil }, wantWarnings: []string{"jobResources"}},
		{name: "Burstable jobs", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobResources.Requests[corev1.ResourceCPU] = resource.MustParse("0")
		}, wantWarnings: []string{"jobResources"}},
		{name: "Burstable CRI-O jobs", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.CRI.CRIService = ptr.To(ytv1.CRIServiceCRIO)
			s.JobResources.Requests[corev1.ResourceMemory] = resource.MustParse("0")
		}, wantWarnings: []string{"jobResources"}},
		{name: "Burstable main allows Burstable containers and round init CPU", modify: func(s *ytv1.ExecNodesSpec) {
			s.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("0")
			s.JobResources = nil
			s.Sidecars = []string{containerTestSpec(t, "sidecar", corev1.ResourceRequirements{})}
			s.InitContainers = []string{containerTestSpec(t, "init", guaranteedTestResources("1"))}
		}},
		{name: "unisolated jobs without dedicated resources", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.Isolated = ptr.To(false)
			s.JobResources = nil
		}},
		{name: "unisolated jobs preserve Guaranteed main", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.Isolated = ptr.To(false)
		}},
		{name: "unisolated Burstable jobs", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.Isolated = ptr.To(false)
			s.JobResources.Requests[corev1.ResourceCPU] = resource.MustParse("0")
		}, wantWarnings: []string{"jobResources"}},
		{name: "unisolated jobs add only CPU", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.Isolated = ptr.To(false)
			delete(s.JobResources.Limits, corev1.ResourceMemory)
			delete(s.JobResources.Requests, corev1.ResourceMemory)
		}},
		{name: "unisolated limits only default after merging", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.Isolated = ptr.To(false)
			s.Resources.Requests = nil
			s.JobResources.Requests = nil
		}},
		{name: "unisolated job requests suppress main request defaulting", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.Isolated = ptr.To(false)
			s.Resources.Requests = nil
		}, wantWarnings: []string{"jobResources"}},
		{name: "jobs without CRI merge into main", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment = nil
			s.JobResources.Requests[corev1.ResourceCPU] = resource.MustParse("0")
		}, wantWarnings: []string{"jobResources"}},
		{name: "disabled CRI service has no isolated jobs container", modify: func(s *ytv1.ExecNodesSpec) {
			s.JobEnvironment.CRI.CRIService = ptr.To(ytv1.CRIServiceNone)
			s.JobResources = nil
		}},
		{name: "sidecar without resources", modify: func(s *ytv1.ExecNodesSpec) {
			s.Sidecars = []string{containerTestSpec(t, "sidecar", corev1.ResourceRequirements{})}
		}, wantWarnings: []string{"sidecars[0].resources"}},
		{name: "sidecar limits only", modify: func(s *ytv1.ExecNodesSpec) {
			resources := guaranteedTestResources("1")
			resources.Requests = nil
			s.Sidecars = []string{containerTestSpec(t, "sidecar", resources)}
		}},
		{name: "init without resources", modify: func(s *ytv1.ExecNodesSpec) {
			s.InitContainers = []string{containerTestSpec(t, "init", corev1.ResourceRequirements{})}
		}, wantWarnings: []string{"initContainers[0].resources"}},
		{name: "round init CPU", modify: func(s *ytv1.ExecNodesSpec) {
			s.InitContainers = []string{containerTestSpec(t, "init", guaranteedTestResources("1"))}
		}, wantWarnings: []string{"initContainers[0].resources.limits.cpu"}},
		{name: "Burstable init resources", modify: func(s *ytv1.ExecNodesSpec) {
			resources := guaranteedTestResources("999m")
			resources.Requests[corev1.ResourceMemory] = resource.MustParse("1Gi")
			s.InitContainers = []string{containerTestSpec(t, "init", resources)}
		}, wantWarnings: []string{"initContainers[0].resources"}},
		{name: "init limits only", modify: func(s *ytv1.ExecNodesSpec) {
			resources := guaranteedTestResources("999m")
			resources.Requests = nil
			s.InitContainers = []string{containerTestSpec(t, "init", resources)}
		}},
		{name: "invalid container YAML", modify: func(s *ytv1.ExecNodesSpec) {
			s.Sidecars = []string{"name: sidecar\nunknownField: value"}
		}, wantErrors: []string{"sidecars[0]"}},
		{name: "duplicate container names", modify: func(s *ytv1.ExecNodesSpec) {
			s.Sidecars = append(s.Sidecars, s.Sidecars[0])
		}, wantErrors: []string{"sidecars[1].name"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			builder := testutil.YtsaurusBuilder{Images: testutil.CurrImages}
			builder.CreateMinimal()
			builder.WithExecNodes()
			builder.WithScheduler()
			spec := &builder.Ytsaurus.Spec.ExecNodes[0]
			spec.Resources = guaranteedTestResources("2")
			spec.Loggers = stderrTestLoggers()
			spec.JobProxyLoggers = stderrTestLoggers()
			spec.JobResources = ptr.To(guaranteedTestResources("4"))
			spec.JobEnvironment = &ytv1.JobEnvironmentSpec{CRI: &ytv1.CRIJobEnvironmentSpec{}}
			spec.Sidecars = []string{containerTestSpec(t, "sidecar", guaranteedTestResources("1"))}
			spec.InitContainers = []string{containerTestSpec(t, "init", guaranteedTestResources("999m"))}
			if testCase.modify != nil {
				testCase.modify(spec)
			}
			original := spec.DeepCopy()
			for _, remote := range []bool{false, true} {
				var results Results
				prefix := "spec.execNodes[0]."
				if remote {
					validator := NewRemoteExecNodesValidator()
					results = validator.Validate(context.Background(), &ytv1.RemoteExecNodes{
						Spec: ytv1.RemoteExecNodesSpec{ExecNodesSpec: *spec},
					}, nil)
					prefix = "spec."
				} else {
					results = NewYtsaurusValidator().validateExecNodes(builder.Ytsaurus)
				}
				require.Len(t, results.Errors, len(testCase.wantErrors), "remote=%v: %v", remote, results.Errors)
				for i, path := range testCase.wantErrors {
					require.Equal(t, prefix+path, results.Errors[i].Field, "remote=%v", remote)
				}
				require.Len(t, results.Warnings, len(testCase.wantWarnings), "remote=%v: %v", remote, results.Warnings)
				for i, path := range testCase.wantWarnings {
					var warning *field.Error
					require.ErrorAs(t, results.Warnings[i], &warning)
					require.Equal(t, prefix+path, warning.Field, "remote=%v", remote)
				}
			}
			require.Equal(t, original, spec, "validation must not mutate resources")
		})
	}
}

func TestValidateMasterSidecarQoS(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		configure   func(*ytv1.MastersSpec)
		wantWarning string
	}{
		{name: "custom sidecar without resources", configure: func(s *ytv1.MastersSpec) {
			s.Sidecars = []string{containerTestSpec(t, "sidecar", corev1.ResourceRequirements{})}
		}, wantWarning: "sidecars[0].resources"},
		{name: "Guaranteed custom sidecar", configure: func(s *ytv1.MastersSpec) {
			s.Sidecars = []string{containerTestSpec(t, "sidecar", guaranteedTestResources("1"))}
		}},
		{name: "uploader without configurable resources", configure: func(s *ytv1.MastersSpec) {
			s.HydraPersistenceUploader = &ytv1.HydraPersistenceUploaderSpec{Image: ptr.To("test")}
		}, wantWarning: "hydraPersistenceUploader"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			for _, guaranteed := range []bool{false, true} {
				builder := testutil.YtsaurusBuilder{Images: testutil.CurrImages}
				builder.CreateMinimal()
				master := &builder.Ytsaurus.Spec.PrimaryMasters
				master.Loggers = stderrTestLoggers()
				if guaranteed {
					master.Resources = guaranteedTestResources("2")
				}
				testCase.configure(&master.MastersSpec)
				results := NewYtsaurusValidator().validatePrimaryMasters(builder.Ytsaurus, nil)
				require.Empty(t, results.Errors)
				if guaranteed && testCase.wantWarning != "" {
					require.Len(t, results.Warnings, 1)
					var warning *field.Error
					require.ErrorAs(t, results.Warnings[0], &warning)
					require.Equal(t, "spec.primaryMasters."+testCase.wantWarning, warning.Field)
				} else {
					require.Empty(t, results.Warnings)
				}
			}
		})
	}
}

func TestRemoteExecNodeQoSAdmissionWarnings(t *testing.T) {
	obj := &ytv1.RemoteExecNodes{Spec: ytv1.RemoteExecNodesSpec{ExecNodesSpec: ytv1.ExecNodesSpec{
		InstanceSpec:   ytv1.InstanceSpec{Resources: guaranteedTestResources("2")},
		JobEnvironment: &ytv1.JobEnvironmentSpec{CRI: &ytv1.CRIJobEnvironmentSpec{}},
		Sidecars:       []string{containerTestSpec(t, "sidecar", corev1.ResourceRequirements{})},
		InitContainers: []string{containerTestSpec(t, "init", guaranteedTestResources("1"))},
	}}}
	validator := NewRemoteExecNodesValidator()
	warnings, err := validator.ValidateCreate(context.Background(), obj)
	require.NoError(t, err)
	require.Len(t, warnings, 3)
	require.Contains(t, warnings[0], "spec.initContainers[0].resources.limits.cpu")
	require.Contains(t, warnings[1], "spec.sidecars[0].resources")
	require.Contains(t, warnings[2], "spec.jobResources")
	updateWarnings, err := validator.ValidateUpdate(context.Background(), obj, obj.DeepCopy())
	require.NoError(t, err)
	require.Equal(t, warnings, updateWarnings)
}

func TestValidateTimbertruckQoS(t *testing.T) {
	validator := baseValidator{}
	for _, guaranteed := range []bool{false, true} {
		for _, legacy := range []bool{false, true} {
			spec := ytv1.InstanceSpec{Loggers: stderrTestLoggers()}
			if guaranteed {
				spec.Resources = guaranteedTestResources("2")
			}
			timbertruck := &ytv1.TimbertruckSpec{Image: ptr.To("test")}
			spec.StructuredLoggers = []ytv1.StructuredLoggerSpec{{
				Category: ptr.To("Access"),
				BaseLoggerSpec: ytv1.BaseLoggerSpec{RotationPolicy: &ytv1.LogRotationPolicy{
					MaxTotalSizeToKeep: ptr.To(resource.MustParse("100Mi")),
					MaxSegmentSize:     ptr.To(resource.MustParse("10Mi")),
				}},
			}}
			var componentTimbertruck *ytv1.TimbertruckSpec
			wantPath := "spec.discovery.structuredLoggers[0].enableDelivery"
			if legacy {
				componentTimbertruck = timbertruck
				wantPath = "spec.discovery.timbertruck"
			} else {
				spec.StructuredLoggers[0].EnableDelivery = ptr.To(true)
			}
			spec.Locations = []ytv1.LocationSpec{{LocationType: ytv1.LocationTypeLogs, Path: "/logs"}}
			spec.VolumeMounts = []corev1.VolumeMount{{Name: "logs", MountPath: "/logs"}}
			results := validator.validateInstanceSpecWithTimbertruck(spec, &ytv1.CommonSpec{Timbertruck: timbertruck}, componentTimbertruck, field.NewPath("spec", "discovery"))
			require.Empty(t, results.Errors)
			if guaranteed {
				require.Len(t, results.Warnings, 1)
				var warning *field.Error
				require.ErrorAs(t, results.Warnings[0], &warning)
				require.Equal(t, wantPath, warning.Field)
			} else {
				require.Empty(t, results.Warnings)
			}
		}
	}
}
