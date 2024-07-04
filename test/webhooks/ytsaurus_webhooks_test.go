package webhooks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
)

var _ = Describe("Test for Ytsaurus webhooks", func() {
	const namespace string = "default"

	Context("When setting up the test environment", func() {
		It("Should not accept a Ytsaurus resource without `default` http proxy role", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.HTTPProxies = []ytv1.HTTPProxiesSpec{
				{
					Role: "not_default",
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("HTTP proxy with `default` role should exist")))
		})

		It("Should not accept a Ytsaurus resource with the same RPC proxies roles", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.RPCProxies = []ytv1.RPCProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.rpcProxies[1].role: Duplicate value: \"default\"")))
		})

		It("Should not accept a Ytsaurus resource with the same TCP proxies roles", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.TCPProxies = []ytv1.TCPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tcpProxies[1].role: Duplicate value: \"default\"")))
		})

		It("Should not accept a Ytsaurus resource with the same HTTP proxies roles", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.HTTPProxies = []ytv1.HTTPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.httpProxies[1].role: Duplicate value: \"default\"")))
		})

		It("Should not accept a Ytsaurus resource with the same node names", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.DataNodes = []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.dataNodes[1].name: Duplicate value: \"default\"")))

			ytsaurus = testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes[1].name: Duplicate value: \"default\"")))

			ytsaurus = testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[1].name: Duplicate value: \"default\"")))
		})

		It("Should not accept a cell tag update", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testutil.YtsaurusName,
				Namespace: namespace,
			}, ytsaurus)).Should(Succeed())

			ytsaurus.Spec.PrimaryMasters.CellTag = 123

			Expect(k8sClient.Update(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.primaryMasters.cellTag")))
		})

		It("Should not accept data nodes without chunk locations", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.DataNodes = []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.dataNodes[0].locations")))
		})

		It("Should not accept a Ytsaurus resource with EnableAntiAffinity flag set in different spec fields", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)

			trueEnableAntiAffinity := true
			falseEnableAntiAffinity := false

			ytsaurus.Spec.DataNodes = []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						EnableAntiAffinity: &trueEnableAntiAffinity,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "123",
								MountPath: "/yt",
							},
						},
						Locations: []ytv1.LocationSpec{
							{
								LocationType: ytv1.LocationTypeChunkStore,
								Path:         "/yt/chunk-store",
							},
						},
					},
					Name: "default",
				},
				{
					InstanceSpec: ytv1.InstanceSpec{
						EnableAntiAffinity: &falseEnableAntiAffinity,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "123",
								MountPath: "/yt",
							},
						},
						Locations: []ytv1.LocationSpec{
							{
								LocationType: ytv1.LocationTypeChunkStore,
								Path:         "/yt/chunk-store",
							},
						},
					},
					Name: "other",
				},
			}
			ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{
				InstanceSpec: ytv1.InstanceSpec{
					EnableAntiAffinity: &trueEnableAntiAffinity,
				},
			}
			ytsaurus.Spec.PrimaryMasters = ytv1.MastersSpec{
				InstanceSpec: ytv1.InstanceSpec{
					EnableAntiAffinity: &trueEnableAntiAffinity,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "123",
							MountPath: "/yt",
						},
					},
					Locations: []ytv1.LocationSpec{
						{
							LocationType: ytv1.LocationTypeMasterSnapshots,
							Path:         "/yt/master-snapshots",
						},
						{
							LocationType: ytv1.LocationTypeMasterChangelogs,
							Path:         "/yt/master-changelogs",
						},
					},
				},
			}

			err := k8sClient.Create(ctx, ytsaurus)
			statusErr, isStatus := err.(*apierrors.StatusError)
			Expect(isStatus).To(BeTrue())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("EnableAntiAffinity is deprecated, use Affinity instead"))

			errorDetails := statusErr.ErrStatus.Details
			Expect(errorDetails.Causes).To(HaveLen(4))
			for _, cause := range errorDetails.Causes {
				Expect(cause.Type).To(Equal(metav1.CauseTypeFieldValueInvalid))
				Expect(cause.Message).To(ContainSubstring("EnableAntiAffinity is deprecated, use Affinity instead"))
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("EnableAntiAffinity is deprecated, use Affinity instead")))
		})

		It("Should not accept invalid Sidecars", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
					Sidecars:     []string{"foo"},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[0].sidecars[0]: Invalid value")))

			ytsaurus = testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
					Sidecars:     []string{"name: foo", "name: foo"},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[0].sidecars[1].name: Duplicate value: \"foo\"")))
		})

		It("Check combination of schedulers, controllerAgents and execNodes", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.Schedulers = nil
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.schedulers")))

			ytsaurus = testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ControllerAgents = nil
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.controllerAgents")))

			ytsaurus = testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.Schedulers = nil
			ytsaurus.Spec.ControllerAgents = nil
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.schedulers: Required value: execNodes doesn't make sense without schedulers")))
		})

		It("Should not accept queryTracker without tabletNodes and scheduler", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.TabletNodes = nil

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes: Required")))

			ytsaurus = testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.Schedulers = nil

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.schedulers: Required")))
		})

		It("Should not accept queueAgent without tabletNodes", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.TabletNodes = nil

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes: Required")))
		})

		It("Check volumeMounts for locations", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.PrimaryMasters.VolumeMounts = []corev1.VolumeMount{}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("location path is not in any volume mount")))
		})

		It("Should not accept non-empty primaryMaster/hostAddresses with HostNetwork=false", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.HostNetwork = false
			ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{"test.yt.address"}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.hostNetwork: Required")))
		})

		It("primaryMaster/hostAddresses length should be equal to instanceCount", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.HostNetwork = true
			ytsaurus.Spec.PrimaryMasters.InstanceCount = 3
			ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{"test.yt.address"}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.primaryMasters.hostAddresses: Invalid value")))
		})

		It("should deny the creation of another YTsaurus CRD in the same namespace", func() {
			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("already exists")))
		})
	})
})
