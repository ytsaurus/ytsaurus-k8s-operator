package v1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Test for Ytsaurus webhooks", func() {
	const namespace string = "default"

	Context("When setting up the test environment", func() {
		It("Should not accept a Ytsaurus resource without `default` http proxy role", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.HTTPProxies = []HTTPProxiesSpec{
				{
					Role: "not_default",
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("HTTP proxy with `default` role should exist")))
		})

		It("Should not accept a Ytsaurus resource with the same RPC proxies roles", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.RPCProxies = []RPCProxiesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.rpcProxies[1].role: Duplicate value: \"default\"")))
		})

		It("Should not accept a Ytsaurus resource with the same TCP proxies roles", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.TCPProxies = []TCPProxiesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tcpProxies[1].role: Duplicate value: \"default\"")))
		})

		It("Should not accept a Ytsaurus resource with the same HTTP proxies roles", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.HTTPProxies = []HTTPProxiesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.httpProxies[1].role: Duplicate value: \"default\"")))
		})

		It("Should not accept a Ytsaurus resource with the same node names", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.DataNodes = []DataNodesSpec{
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.dataNodes[1].name: Duplicate value: \"default\"")))

			ytsaurus = CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.TabletNodes = []TabletNodesSpec{
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes[1].name: Duplicate value: \"default\"")))

			ytsaurus = CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ExecNodes = []ExecNodesSpec{
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[1].name: Duplicate value: \"default\"")))
		})

		It("Should not accept a cell tag update", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      YtsaurusName,
				Namespace: namespace,
			}, ytsaurus)).Should(Succeed())

			ytsaurus.Spec.PrimaryMasters.CellTag = 123

			Expect(k8sClient.Update(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.primaryMasters.cellTag")))
		})

		It("Should not accept data nodes without chunk locations", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.DataNodes = []DataNodesSpec{
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.dataNodes[0].locations")))
		})

		It("Should not accept a Ytsaurus resource with EnableAntiAffinity flag set in different spec fields", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)

			trueEnableAntiAffinity := true
			falseEnableAntiAffinity := false

			ytsaurus.Spec.DataNodes = []DataNodesSpec{
				{
					InstanceSpec: InstanceSpec{
						EnableAntiAffinity: &trueEnableAntiAffinity,
						Locations: []LocationSpec{
							{
								LocationType: LocationTypeChunkStore,
								Path:         "/yt/chunk-store",
							},
						},
					},
					Name: "default",
				},
				{
					InstanceSpec: InstanceSpec{
						EnableAntiAffinity: &falseEnableAntiAffinity,
						Locations: []LocationSpec{
							{
								LocationType: LocationTypeChunkStore,
								Path:         "/yt/chunk-store",
							},
						},
					},
					Name: "other",
				},
			}
			ytsaurus.Spec.ControllerAgents = &ControllerAgentsSpec{
				InstanceSpec: InstanceSpec{
					EnableAntiAffinity: &trueEnableAntiAffinity,
				},
			}
			ytsaurus.Spec.PrimaryMasters = MastersSpec{
				InstanceSpec: InstanceSpec{
					EnableAntiAffinity: &trueEnableAntiAffinity,
					Locations: []LocationSpec{
						{
							LocationType: LocationTypeMasterSnapshots,
							Path:         "/yt/master-snapshots",
						},
						{
							LocationType: LocationTypeMasterChangelogs,
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
			Expect(len(errorDetails.Causes)).To(Equal(4))
			for _, cause := range errorDetails.Causes {
				Expect(cause.Type).To(Equal(metav1.CauseTypeFieldValueInvalid))
				Expect(cause.Message).To(ContainSubstring("EnableAntiAffinity is deprecated, use Affinity instead"))
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("EnableAntiAffinity is deprecated, use Affinity instead")))
		})

		It("Should not accept invalid Sidecars", func() {
			ytsaurus := CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ExecNodes = []ExecNodesSpec{
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
					Sidecars:     []string{"foo"},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[0].sidecars[0]: Invalid value")))

			ytsaurus = CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.ExecNodes = []ExecNodesSpec{
				{
					InstanceSpec: InstanceSpec{InstanceCount: 1},
					Sidecars:     []string{"name: foo", "name: foo"},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[0].sidecars[1].name: Duplicate value: \"foo\"")))
		})
	})
})
