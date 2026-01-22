package webhooks

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("Test for Ytsaurus webhooks", func() {
	const namespace string = "default"
	var builder *testutil.YtsaurusBuilder
	var ytsaurus *ytv1.Ytsaurus

	newYtsaurus := func() *ytv1.Ytsaurus {
		builder = &testutil.YtsaurusBuilder{
			Images:    testutil.CurrentImages,
			Namespace: namespace,
		}
		By("Creating new ytsaurus spec")
		builder.CreateMinimal()
		builder.WithBaseComponents()
		return builder.Ytsaurus
	}

	Context("When setting up the test environment", func() {
		BeforeEach(func() {
			ytsaurus = newYtsaurus()
		})

		Context("Secure cluster transports", func() {
			BeforeEach(func() {
				builder.WithRPCProxies()
				ytsaurus.Spec.ClusterFeatures = &ytv1.ClusterFeatures{
					RPCProxyHavePublicAddress: true,
					HTTPProxyHaveHTTPSAddress: true,
					SecureClusterTransports:   true,
				}
				ytsaurus.Spec.NativeTransport = &ytv1.RPCTransportSpec{
					TLSSecret: &corev1.LocalObjectReference{
						Name: "server-cert",
					},
					TLSClientSecret: &corev1.LocalObjectReference{
						Name: "client-cert",
					},
					TLSRequired: true,
					TLSInsecure: false,
				}
				ytsaurus.Spec.HTTPProxies[0].Transport = ytv1.HTTPTransportSpec{
					HTTPSSecret: &corev1.LocalObjectReference{
						Name: "https-secret",
					},
					DisableHTTP: true,
				}
				ytsaurus.Spec.RPCProxies[0].Transport = ytv1.RPCTransportSpec{
					TLSSecret: &corev1.LocalObjectReference{
						Name: "rpc-proxy-secret",
					},
					TLSRequired: true,
				}

				By("Checking success", func() {
					ytsaurus1 := ytsaurus.DeepCopy()
					Expect(k8sClient.Create(ctx, ytsaurus1)).Should(Succeed())
					Expect(k8sClient.Delete(ctx, ytsaurus1)).Should(Succeed())
				})
			})

			AfterEach(func() {
				ytsaurus1 := ytsaurus.DeepCopy()
				By("Checking failure", func() {
					err := k8sClient.Create(ctx, ytsaurus)
					Expect(err).ShouldNot(Succeed())
				})
				By("Checking success without secure cluster", func() {
					ytsaurus1.Spec.ClusterFeatures.HTTPProxyHaveHTTPSAddress = false
					ytsaurus1.Spec.ClusterFeatures.SecureClusterTransports = false
					Expect(k8sClient.Create(ctx, ytsaurus1)).Should(Succeed())
					Expect(k8sClient.Delete(ctx, ytsaurus1)).Should(Succeed())
				})
			})

			DescribeTable("Failures",
				func(fn func()) {
					fn()
				},
				Entry("no rpc public address", func() {
					ytsaurus.Spec.ClusterFeatures.RPCProxyHavePublicAddress = false
				}),
				Entry("no https proxy address", func() {
					ytsaurus.Spec.ClusterFeatures.HTTPProxyHaveHTTPSAddress = false
				}),
				Entry("no tls setup", func() {
					ytsaurus.Spec.NativeTransport = nil
				}),
				Entry("tls optional", func() {
					ytsaurus.Spec.NativeTransport.TLSRequired = false
				}),
				Entry("tls insecure", func() {
					ytsaurus.Spec.NativeTransport.TLSInsecure = true
				}),
				Entry("no client cert", func() {
					ytsaurus.Spec.NativeTransport.TLSClientSecret = nil
					ytsaurus.Spec.NativeTransport.TLSInsecure = true
				}),
				Entry("instance tls optional", func() {
					transport := *ytsaurus.Spec.NativeTransport
					transport.TLSRequired = false
					ytsaurus.Spec.PrimaryMasters.NativeTransport = &transport
				}),
				Entry("instance tls insecure", func() {
					transport := *ytsaurus.Spec.NativeTransport
					transport.TLSInsecure = true
					ytsaurus.Spec.PrimaryMasters.NativeTransport = &transport
				}),
				Entry("no https", func() {
					ytsaurus.Spec.HTTPProxies[0].Transport = ytv1.HTTPTransportSpec{}
				}),
				Entry("not https-only", func() {
					ytsaurus.Spec.HTTPProxies[0].Transport.DisableHTTP = false
				}),
				Entry("rpc-proxy without tls", func() {
					ytsaurus.Spec.RPCProxies[0].Transport = ytv1.RPCTransportSpec{}
				}),
				Entry("rpc-proxy not tls-only", func() {
					ytsaurus.Spec.RPCProxies[0].Transport.TLSRequired = false
				}),
			)
		})

		It("Should not accept feature httpProxyHaveHttpsAddress without httpsSecret", func() {
			ytsaurus.Spec.ClusterFeatures = &ytv1.ClusterFeatures{
				HTTPProxyHaveHTTPSAddress: true,
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("Cluster feature httpProxyHaveHttpsAddress requires HTTPS for all HTTP proxies")))
		})

		It("Should not accept a Ytsaurus resource without `default` http proxy role", func() {
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
			ytsaurus.Spec.DataNodes = []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.dataNodes[1].name: Duplicate value: \"default\"")))

			ytsaurus = newYtsaurus()
			ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes[1].name: Duplicate value: \"default\"")))

			ytsaurus = newYtsaurus()
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
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      testutil.YtsaurusName,
				Namespace: namespace,
			}, ytsaurus)).Should(Succeed())

			ytsaurus.Spec.PrimaryMasters.CellTag = 123

			Expect(k8sClient.Update(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.primaryMasters.cellTag")))
			Expect(k8sClient.Delete(ctx, ytsaurus)).Should(Succeed())
		})

		It("Should not accept data nodes without chunk locations", func() {
			ytsaurus.Spec.DataNodes = []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.dataNodes[0].locations")))
		})

		It("Should not accept a Ytsaurus resource with EnableAntiAffinity flag set in different spec fields", func() {
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
			statusErr := &apierrors.StatusError{}
			isStatus := errors.As(err, &statusErr)
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
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
					Sidecars:     []string{"foo"},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[0].sidecars[0]: Invalid value")))

			ytsaurus = newYtsaurus()
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
					Sidecars:     []string{"name: foo", "name: foo"},
				},
			}
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.execNodes[0].sidecars[1].name: Duplicate value: \"foo\"")))
		})

		It("Check combination of schedulers, controllerAgents and execNodes", func() {
			ytsaurus.Spec.Schedulers = nil
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.schedulers")))

			ytsaurus = newYtsaurus()
			ytsaurus.Spec.ControllerAgents = nil
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.controllerAgents")))

			ytsaurus = newYtsaurus()
			ytsaurus.Spec.Schedulers = nil
			ytsaurus.Spec.ControllerAgents = nil
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.schedulers: Required value: execNodes doesn't make sense without schedulers")))
		})

		It("Should not accept queryTracker without tabletNodes and scheduler", func() {
			ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.TabletNodes = nil

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes: Required")))

			ytsaurus = newYtsaurus()
			ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.Schedulers = nil

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.schedulers: Required")))
		})

		It("Should not accept queueAgent without tabletNodes", func() {
			ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.TabletNodes = nil

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.tabletNodes: Required")))
		})

		It("Check volumeMounts for locations", func() {
			ytsaurus.Spec.PrimaryMasters.VolumeMounts = []corev1.VolumeMount{}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("location path is not in any volume mount")))
		})

		It("Should not accept non-empty primaryMaster/hostAddresses with HostNetwork=false", func() {
			ytsaurus.Spec.HostNetwork = false
			ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{"test.yt.address"}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.hostNetwork: Required")))
		})

		It("primaryMaster/hostAddresses length should be equal to instanceCount", func() {
			ytsaurus.Spec.HostNetwork = true
			ytsaurus.Spec.PrimaryMasters.InstanceCount = 3
			ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{"test.yt.address"}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.primaryMasters.hostAddresses: Invalid value")))
		})

		It("should deny the creation of another YTsaurus CRD in the same namespace", func() {
			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			ytsaurus1 := newYtsaurus()
			Expect(k8sClient.Create(ctx, ytsaurus1)).Should(MatchError(ContainSubstring("already exists")))
			ytsaurus1.Name += "1"
			Expect(k8sClient.Create(ctx, ytsaurus1)).Should(MatchError(ContainSubstring("already exists")))
			Expect(k8sClient.Delete(ctx, ytsaurus)).Should(Succeed())
		})

		It("Should not accept Timbertruck without structured loggers, logs location and Image", func() {
			ytsaurus.Spec.PrimaryMasters.Timbertruck = &ytv1.TimbertruckSpec{}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(SatisfyAll(
				ContainSubstring("spec.primaryMasters.timbertruck.image: Required value: timbertruck image is required"),
				ContainSubstring("spec.primaryMasters.structuredLoggers: Required value: structuredLoggers must be configured when timbertruck is enabled"),
				ContainSubstring("spec.primaryMasters.locations: Required value: logs location must be configured when timbertruck is enabled"),
			)))
		})

		It("Should accept Timbertruck with structured loggers, logs location and Image", func() {
			image := "ghcr.io/ytsaurus/sidecars:0.0.0"
			ytsaurus.Spec.PrimaryMasters.Timbertruck = &ytv1.TimbertruckSpec{Image: &image}
			ytsaurus.Spec.PrimaryMasters.StructuredLoggers = []ytv1.StructuredLoggerSpec{{BaseLoggerSpec: ytv1.BaseLoggerSpec{Name: "access"}, Category: "Access"}}
			ytsaurus.Spec.PrimaryMasters.Locations = append(ytsaurus.Spec.PrimaryMasters.Locations, ytv1.LocationSpec{LocationType: ytv1.LocationTypeLogs, Path: "/yt/master-logs"})
			ytsaurus.Spec.PrimaryMasters.VolumeMounts = append(ytsaurus.Spec.PrimaryMasters.VolumeMounts, corev1.VolumeMount{Name: "master-logs", MountPath: "/yt/master-logs"})

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, ytsaurus)).Should(Succeed())
		})

		DescribeTable("Forbidden update plans",
			func(plan []ytv1.ComponentUpdateSelector, msg string) {
				ytsaurus.Spec.UpdatePlan = plan
				Expect(k8sClient.Create(ctx, ytsaurus)).To(MatchError(ContainSubstring(msg)))
			},
			Entry("empty entry", []ytv1.ComponentUpdateSelector{
				{},
			}, "Either component or class should be specified"),
			Entry("both class and component", []ytv1.ComponentUpdateSelector{
				{Class: consts.ComponentClassStateless, Component: ytv1.Component{Type: consts.MasterType}},
			}, "Only one of component or class should be specified"),
			Entry("unknown type", []ytv1.ComponentUpdateSelector{
				{Component: ytv1.Component{Type: "Unknown"}},
			}, "Unknown component type"),
			Entry("unknown class", []ytv1.ComponentUpdateSelector{
				{Class: "Unknown"},
			}, "Unsupported value"),
			Entry("nothing, something", []ytv1.ComponentUpdateSelector{
				{Class: consts.ComponentClassNothing},
				{Component: ytv1.Component{Type: consts.MasterType}},
			}, "Update plan already contains class Nothing"),
			Entry("everything, something", []ytv1.ComponentUpdateSelector{
				{Class: consts.ComponentClassEverything},
				{Component: ytv1.Component{Type: consts.MasterType}},
			}, "Update plan already contains class Everything"),
			Entry("something, nothing", []ytv1.ComponentUpdateSelector{
				{Component: ytv1.Component{Type: consts.MasterType}},
				{Class: consts.ComponentClassNothing},
			}, "Should be first and only entry"),
			Entry("something, everything", []ytv1.ComponentUpdateSelector{
				{Component: ytv1.Component{Type: consts.MasterType}},
				{Class: consts.ComponentClassEverything},
			}, "Should be first and only entry"),
			Entry("everything, rollingUpdate", []ytv1.ComponentUpdateSelector{
				{Class: consts.ComponentClassEverything,
					Strategy: &ytv1.ComponentUpdateStrategy{
						RollingUpdate: &ytv1.ComponentRollingUpdateMode{
							BatchSize: ptr.To(int32(1)),
						},
					},
				},
			}, "Everything class supports only BulkUpdate mode"),
			Entry("statefulAgents, onDelete", []ytv1.ComponentUpdateSelector{
				{
					Class: consts.ComponentClassStatefulAgents,
					Strategy: &ytv1.ComponentUpdateStrategy{
						OnDelete: &ytv1.ComponentOnDeleteUpdateMode{},
					},
				},
			}, "StatefulAgents supports only BulkUpdate mode"),
		)

		It("Should accept class Stateless update plan with strategy", func() {
			ytsaurus := newYtsaurus()
			ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{
				{
					Class: consts.ComponentClassStateless,
					Strategy: &ytv1.ComponentUpdateStrategy{
						RunPreChecks: ptr.To(true),
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, ytsaurus)).Should(Succeed())
		})

		It("Should accept component update without updateMode (backward compatibility)", func() {
			ytsaurus := newYtsaurus()
			ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
			ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
				{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}},
			}
			ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{
				{
					Component: ytv1.Component{Type: consts.QueueAgentType},
					// No UpdateMode specified - allowed for backward compatibility
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, ytsaurus)).Should(Succeed())
		})

		DescribeTable("Allowed BulkUpdate modes",
			func(componentType consts.ComponentType) {
				ytsaurus := newYtsaurus()
				// Set up required component specs for validation
				switch componentType {
				case consts.QueryTrackerType:
					ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
					ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
						{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}},
					}
				case consts.QueueAgentType:
					ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
					ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
						{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}},
					}
				case consts.YqlAgentType:
					ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
					ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
					ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
						{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}},
					}
				}
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{
					{
						Component: ytv1.Component{Type: componentType},
						Strategy:  &ytv1.ComponentUpdateStrategy{},
					},
				}

				Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, ytsaurus)).Should(Succeed())
			},
			Entry("QueryTracker", consts.QueryTrackerType),
			Entry("YqlAgent", consts.YqlAgentType),
			Entry("QueueAgent", consts.QueueAgentType),
		)

		DescribeTable("Forbidden RollingUpdate modes",
			func(componentType consts.ComponentType, strategy ytv1.ComponentUpdateStrategy, expectedErrorMsg string) {
				ytsaurus := newYtsaurus()

				// Set up required component specs for validation
				switch componentType {
				case consts.QueryTrackerType:
					ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
					ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
						{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}},
					}
				case consts.QueueAgentType:
					ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
					ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
						{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}},
					}
				case consts.YqlAgentType:
					ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}
				}

				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{
					{
						Component: ytv1.Component{Type: componentType},
						Strategy:  &strategy,
					},
				}

				Expect(k8sClient.Create(ctx, ytsaurus)).ShouldNot(Succeed())
			},
			Entry("Master", consts.MasterType, ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}, "Master doesn't support RollingUpdate mode"),
			Entry("Scheduler", consts.SchedulerType, ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}, "Scheduler doesn't support RollingUpdate mode"),
			Entry("QueryTracker", consts.QueryTrackerType, ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}, "QueryTracker supports only BulkUpdate mode"),
			Entry("QueryTracker", consts.QueryTrackerType, ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}}, "QueryTracker supports only BulkUpdate mode"),
			Entry("QueueAgent", consts.QueueAgentType, ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}, "QueueAgent supports only BulkUpdate mode"),
			Entry("QueueAgent", consts.QueueAgentType, ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}}, "QueueAgent supports only BulkUpdate mode"),
			Entry("YqlAgent", consts.YqlAgentType, ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}}, "YqlAgent supports only BulkUpdate mode"),
			Entry("YqlAgent", consts.YqlAgentType, ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}, "YqlAgent supports only BulkUpdate mode"),
		)
	})
})
