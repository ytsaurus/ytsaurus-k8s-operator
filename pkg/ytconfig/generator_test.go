package ytconfig

import (
	"testing"

	"go.ytsaurus.tech/library/go/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/canonize"
)

var (
	testLogRotationPeriod   int64 = 900000
	testTotalLogSize              = 10 * int64(1<<30)
	testMasterExternalHosts       = []string{
		"host1.external.address",
		"host2.external.address",
		"host3.external.address",
	}
)

func TestGetMasterConfig(t *testing.T) {
	t.Helper()

	g := NewGenerator(getTestYtsaurus(), "fake.zone")
	mc, _ := g.GetMasterConfig()

	canonize.Assert(t, mc)
}

func TestGetMasterWithFixedHostsConfig(t *testing.T) {
	t.Helper()

	ytsaurus := getTestYtsaurusWithFixedMasterHosts()
	g := NewGenerator(ytsaurus, "fake.zone")
	mc, _ := g.GetMasterConfig()

	canonize.Assert(t, mc)
}

func TestGetSchedulerConfig(t *testing.T) {
	t.Helper()

	ytsaurus := getTestYtsaurus()
	ytsaurus.Spec.Schedulers = &v1.SchedulersSpec{
		InstanceSpec: v1.InstanceSpec{InstanceCount: 1},
	}
	g := NewGenerator(ytsaurus, "fake.zone")
	mc, _ := g.GetSchedulerConfig()

	canonize.Assert(t, mc)
}

func TestGetSchedulerWithFixedMasterHostsConfig(t *testing.T) {
	t.Helper()

	ytsaurus := getTestYtsaurusWithFixedMasterHosts()
	ytsaurus.Spec.Schedulers = &v1.SchedulersSpec{
		InstanceSpec: v1.InstanceSpec{InstanceCount: 1},
	}
	g := NewGenerator(ytsaurus, "fake.zone")
	mc, _ := g.GetSchedulerConfig()

	canonize.Assert(t, mc)
}

func getTestYtsaurus() *v1.Ytsaurus {
	return &v1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake",
			Name:      "test",
		},
		Spec: v1.YtsaurusSpec{
			UseIPv6: true,

			PrimaryMasters: v1.MastersSpec{
				CellTag:                0,
				MaxSnapshotCountToKeep: ptr.Int(1543),
				InstanceSpec: v1.InstanceSpec{
					InstanceCount: 1,

					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "master-data",
							MountPath: "/yt/master-data",
						},
					},

					Locations: []v1.LocationSpec{
						{
							LocationType: v1.LocationTypeMasterChangelogs,
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: v1.LocationTypeMasterSnapshots,
							Path:         "/yt/master-data/master-snapshots",
						},
					},

					VolumeClaimTemplates: []v1.EmbeddedPersistentVolumeClaim{
						{
							EmbeddedObjectMetadata: v1.EmbeddedObjectMetadata{
								Name: "master-data",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("20Gi"),
									},
								},
							},
						},
					},

					Loggers: []v1.TextLoggerSpec{
						{
							BaseLoggerSpec: v1.BaseLoggerSpec{
								Name:        "info",
								MinLogLevel: v1.LogLevelInfo,
								Compression: v1.LogCompressionNone,
								Format:      v1.LogFormatPlainText,
							},
							WriterType: v1.LogWriterTypeFile,
						},
						{
							BaseLoggerSpec: v1.BaseLoggerSpec{
								Name:        "error",
								MinLogLevel: v1.LogLevelError,
								Compression: v1.LogCompressionNone,
								Format:      v1.LogFormatPlainText,
							},
							WriterType: v1.LogWriterTypeFile,
						},
						{
							BaseLoggerSpec: v1.BaseLoggerSpec{
								Name:        "debug",
								MinLogLevel: v1.LogLevelDebug,
								Compression: v1.LogCompressionZstd,
								Format:      v1.LogFormatPlainText,

								RotationPolicy: &v1.LogRotationPolicy{
									RotationPeriodMilliseconds: &testLogRotationPeriod,
									MaxTotalSizeToKeep:         &testTotalLogSize,
								},
							},
							WriterType: v1.LogWriterTypeFile,
							CategoriesFilter: &v1.CategoriesFilter{
								Type:   v1.CategoriesFilterTypeExclude,
								Values: []string{"Bus"},
							},
						},
					},
				},
			},
		},
	}
}

func getTestYtsaurusWithFixedMasterHosts() *v1.Ytsaurus {
	ytsaurus := getTestYtsaurus()
	ytsaurus.Spec.PrimaryMasters.HostAddresses = testMasterExternalHosts
	return ytsaurus
}
