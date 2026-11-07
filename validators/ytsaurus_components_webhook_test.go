package validators

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

func componentTestObjects(kind string) (*ytv1.Ytsaurus, client.Object, *ytv1.InstanceSpec) {
	builder := &testutil.YtsaurusBuilder{Images: testutil.CurrImages, Namespace: "test"}
	builder.CreateMinimal()
	parent := builder.Ytsaurus
	parent.Name = "cluster"
	parent.Spec.HTTPProxies[0].Role = "default"
	parent.Spec.Schedulers = &ytv1.SchedulersSpec{}
	parent.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{}
	meta := metav1.ObjectMeta{Name: "component", Namespace: parent.Namespace}
	reference := ytv1.YtsaurusReference{Name: parent.Name}
	instance := ytv1.InstanceSpec{
		InstanceCount: 1,
		VolumeMounts:  []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
		Locations: []ytv1.LocationSpec{
			{LocationType: ytv1.LocationTypeChunkStore, Path: "/data/store"},
			{LocationType: ytv1.LocationTypeChunkCache, Path: "/data/cache"},
			{LocationType: ytv1.LocationTypeSlots, Path: "/data/slots"},
		},
	}
	switch kind {
	case "YtsaurusDataNodes":
		obj := &ytv1.YtsaurusDataNodes{ObjectMeta: meta, Spec: ytv1.YtsaurusDataNodesSpec{
			Ytsaurus:      reference,
			DataNodesSpec: ytv1.DataNodesSpec{Name: "separate", InstanceSpec: instance},
		}}
		return parent, obj, &obj.Spec.InstanceSpec
	case "YtsaurusExecNodes":
		obj := &ytv1.YtsaurusExecNodes{ObjectMeta: meta, Spec: ytv1.YtsaurusExecNodesSpec{
			Ytsaurus:      reference,
			ExecNodesSpec: ytv1.ExecNodesSpec{Name: "separate", InstanceSpec: instance},
		}}
		return parent, obj, &obj.Spec.InstanceSpec
	case "YtsaurusTabletNodes":
		obj := &ytv1.YtsaurusTabletNodes{ObjectMeta: meta, Spec: ytv1.YtsaurusTabletNodesSpec{
			Ytsaurus:        reference,
			TabletNodesSpec: ytv1.TabletNodesSpec{Name: "separate", InstanceSpec: instance},
		}}
		return parent, obj, &obj.Spec.InstanceSpec
	case "YtsaurusHTTPProxies":
		obj := &ytv1.YtsaurusHTTPProxies{ObjectMeta: meta, Spec: ytv1.YtsaurusHTTPProxiesSpec{
			Ytsaurus:        reference,
			HTTPProxiesSpec: ytv1.HTTPProxiesSpec{Role: "separate", InstanceSpec: instance},
		}}
		return parent, obj, &obj.Spec.InstanceSpec
	case "YtsaurusRPCProxies":
		obj := &ytv1.YtsaurusRPCProxies{ObjectMeta: meta, Spec: ytv1.YtsaurusRPCProxiesSpec{
			Ytsaurus:       reference,
			RPCProxiesSpec: ytv1.RPCProxiesSpec{Role: "separate", InstanceSpec: instance},
		}}
		return parent, obj, &obj.Spec.InstanceSpec
	default:
		panic("unknown component kind: " + kind)
	}
}

func clusterTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, ytv1.AddToScheme(scheme))
	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...)
	return builder.Build()
}

func componentTestValidator(t *testing.T, obj client.Object, objects ...client.Object) admission.CustomValidator {
	t.Helper()
	c := clusterTestClient(t, objects...)
	switch obj.(type) {
	case *ytv1.YtsaurusDataNodes:
		return componentTestClient(NewYtsaurusDataNodesValidator(), c)
	case *ytv1.YtsaurusExecNodes:
		return componentTestClient(NewYtsaurusExecNodesValidator(), c)
	case *ytv1.YtsaurusTabletNodes:
		return componentTestClient(NewYtsaurusTabletNodesValidator(), c)
	case *ytv1.YtsaurusHTTPProxies:
		return componentTestClient(NewYtsaurusHTTPProxiesValidator(), c)
	case *ytv1.YtsaurusRPCProxies:
		return componentTestClient(NewYtsaurusRPCProxiesValidator(), c)
	default:
		panic("unknown component object")
	}
}

func componentTestClient[T client.Object](validator *customValidator[T], c client.Client) admission.CustomValidator {
	validator.Client = c
	validator.APIReader = c
	return validator
}

func requireComponentError(t *testing.T, validator admission.CustomValidator, obj client.Object, kind, path string) {
	t.Helper()
	_, oldObj, _ := componentTestObjects(kind)
	_, createErr := validator.ValidateCreate(t.Context(), obj)
	_, updateErr := validator.ValidateUpdate(t.Context(), oldObj, obj)
	require.Error(t, createErr)
	require.Error(t, updateErr)
}

func TestYtsaurusComponentValidators(t *testing.T) {
	for _, kind := range []string{"YtsaurusDataNodes", "YtsaurusExecNodes", "YtsaurusTabletNodes", "YtsaurusHTTPProxies", "YtsaurusRPCProxies"} {
		t.Run(kind, func(t *testing.T) {
			t.Run("accepts create and update without conflicting with itself", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				validator := componentTestValidator(t, obj, parent)
				_, err := validator.ValidateCreate(t.Context(), obj)
				require.NoError(t, err)
				validator = componentTestValidator(t, obj, parent, obj)
				_, err = validator.ValidateUpdate(t.Context(), obj.DeepCopyObject(), obj)
				require.NoError(t, err)
			})
			t.Run("validates the incoming spec instead of the stored spec", func(t *testing.T) {
				parent, obj, instance := componentTestObjects(kind)
				instance.VolumeMounts[0].MountPath += "/"
				stored := obj.DeepCopyObject().(client.Object)
				instance.VolumeMounts[0].MountPath = "/data"
				validator := componentTestValidator(t, obj, parent, stored)
				_, err := validator.ValidateUpdate(t.Context(), stored, obj)
				require.NoError(t, err)
			})
			t.Run("allows deletion after parent removal", func(t *testing.T) {
				_, obj, _ := componentTestObjects(kind)
				validator := componentTestValidator(t, obj)
				_, err := validator.ValidateDelete(t.Context(), obj)
				require.NoError(t, err)
			})
			t.Run("allows deletion with an existing parent", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				validator := componentTestValidator(t, obj, parent, obj)
				_, err := validator.ValidateDelete(t.Context(), obj)
				require.NoError(t, err)
			})
			t.Run("allows create and update before parent creation", func(t *testing.T) {
				_, obj, _ := componentTestObjects(kind)
				validator := componentTestValidator(t, obj)
				_, err := validator.ValidateCreate(t.Context(), obj)
				require.NoError(t, err)
				_, err = validator.ValidateUpdate(t.Context(), obj.DeepCopyObject(), obj)
				require.NoError(t, err)
			})
			t.Run("ignores a parent in another namespace", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				parent.Namespace = "other"
				validator := componentTestValidator(t, obj, parent)
				_, err := validator.ValidateCreate(t.Context(), obj)
				require.NoError(t, err)
				_, err = validator.ValidateUpdate(t.Context(), obj.DeepCopyObject(), obj)
				require.NoError(t, err)
			})
			t.Run("requires a parent name", func(t *testing.T) {
				_, obj, _ := componentTestObjects(kind)
				data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
				require.NoError(t, err)
				data["spec"].(map[string]interface{})["ytsaurus"] = map[string]interface{}{"name": ""}
				require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(data, obj))
				requireComponentError(t, componentTestValidator(t, obj), obj, kind, "spec.ytsaurus.name")
			})
			t.Run("validates instance paths on create and update", func(t *testing.T) {
				parent, obj, instance := componentTestObjects(kind)
				instance.VolumeMounts[0].MountPath += "/"
				requireComponentError(t, componentTestValidator(t, obj, parent), obj, kind, "spec.volumeMounts[0]")
			})
			t.Run("validates structured loggers", func(t *testing.T) {
				parent, obj, instance := componentTestObjects(kind)
				instance.StructuredLoggers = []ytv1.StructuredLoggerSpec{{}}
				requireComponentError(t, componentTestValidator(t, obj, parent), obj, kind, "spec.structuredLoggers[0]")
			})
			t.Run("inherits the timbertruck image", func(t *testing.T) {
				parent, obj, instance := componentTestObjects(kind)
				parent.Spec.Timbertruck = &ytv1.TimbertruckSpec{Image: ptr.To("timbertruck:latest")}
				instance.StructuredLoggers = []ytv1.StructuredLoggerSpec{{Category: ptr.To("Access"), EnableDelivery: ptr.To(true)}}
				instance.Locations = append(instance.Locations, ytv1.LocationSpec{LocationType: ytv1.LocationTypeLogs, Path: "/data/logs"})
				_, err := componentTestValidator(t, obj, parent).ValidateCreate(t.Context(), obj)
				require.NoError(t, err)
				parent.Spec.Timbertruck = nil
				requireComponentError(t, componentTestValidator(t, obj, parent), obj, kind, "spec.timbertruck.image")
			})
			t.Run("inherits native TLS settings", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				parent.Spec.NativeTransport = &ytv1.RPCTransportSpec{TLSRequired: true}
				requireComponentError(t, componentTestValidator(t, obj, parent), obj, kind, "spec.nativeTransport.tlsSecret")
				parent.Spec.NativeTransport.TLSSecret = &corev1.LocalObjectReference{Name: "server"}
				parent.Spec.NativeTransport.TLSClientSecret = &corev1.LocalObjectReference{Name: "client"}
				_, err := componentTestValidator(t, obj, parent).ValidateCreate(t.Context(), obj)
				require.NoError(t, err)
			})
			t.Run("rejects duplicate inline names and roles", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				parent.Spec.DataNodes = []ytv1.DataNodesSpec{{Name: "separate"}}
				parent.Spec.ExecNodes = []ytv1.ExecNodesSpec{{Name: "separate"}}
				parent.Spec.TabletNodes = []ytv1.TabletNodesSpec{{Name: "separate"}}
				parent.Spec.HTTPProxies = []ytv1.HTTPProxiesSpec{{Role: "separate"}}
				parent.Spec.RPCProxies = []ytv1.RPCProxiesSpec{{Role: "separate"}}
				path := "spec.name"
				if kind == "YtsaurusHTTPProxies" || kind == "YtsaurusRPCProxies" {
					path = "spec.role"
				}
				requireComponentError(t, componentTestValidator(t, obj, parent), obj, kind, path)
			})
			t.Run("rejects duplicate separate names and roles", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				other := obj.DeepCopyObject().(client.Object)
				other.SetName("another-resource")
				path := "spec.name"
				if kind == "YtsaurusHTTPProxies" || kind == "YtsaurusRPCProxies" {
					path = "spec.role"
				}
				requireComponentError(t, componentTestValidator(t, obj, parent, other), obj, kind, path)
			})
			t.Run("ignores names and roles in another namespace or cluster", func(t *testing.T) {
				parent, obj, _ := componentTestObjects(kind)
				otherNamespace := obj.DeepCopyObject().(client.Object)
				otherNamespace.SetNamespace("other")
				otherCluster := obj.DeepCopyObject().(client.Object)
				otherCluster.SetName("another-cluster-resource")
				data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(otherCluster)
				require.NoError(t, err)
				data["spec"].(map[string]interface{})["ytsaurus"] = map[string]interface{}{"name": "other"}
				require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(data, otherCluster))
				validator := componentTestValidator(t, obj, parent, otherNamespace, otherCluster)
				_, err = validator.ValidateCreate(t.Context(), obj)
				require.NoError(t, err)
			})
		})
	}
}

func TestYtsaurusComponentSpecificValidation(t *testing.T) {
	t.Run("reports deletion validation errors as warnings", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
		parent.Spec.HTTPProxies = nil
		obj.(*ytv1.YtsaurusHTTPProxies).Spec.Role = "default"
		validator := componentTestValidator(t, obj, parent, obj)
		warnings, err := validator.ValidateDelete(t.Context(), obj)
		require.NoError(t, err)
		require.NotEmpty(t, warnings)
		require.Contains(t, warnings[0], "spec.httpProxies")
	})
	t.Run("reports component fetch errors as deletion warnings", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
		obj.(*ytv1.YtsaurusHTTPProxies).Spec.Role = "default"
		validator := componentTestValidator(t, obj, parent, obj)
		warnings, err := validator.ValidateDelete(t.Context(), obj)
		require.NoError(t, err)
		require.Len(t, warnings, 1)
		require.Contains(t, warnings[0], "failed to fetch cluster components: duplicate component")
	})
	t.Run("data nodes need chunk storage", func(t *testing.T) {
		parent, obj, instance := componentTestObjects("YtsaurusDataNodes")
		instance.Locations = nil
		requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusDataNodes", "spec.locations")
	})
	t.Run("exec nodes need chunk cache and slots", func(t *testing.T) {
		for _, locationType := range []ytv1.LocationType{ytv1.LocationTypeChunkCache, ytv1.LocationTypeSlots} {
			parent, obj, instance := componentTestObjects("YtsaurusExecNodes")
			instance.Locations = []ytv1.LocationSpec{{LocationType: locationType, Path: "/data/location"}}
			requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusExecNodes", "spec.locations")
		}
	})
	t.Run("exec nodes need schedulers", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusExecNodes")
		parent.Spec.Schedulers = nil
		requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusExecNodes", "spec.ytsaurus.name")
	})
	t.Run("exec node sidecars and init containers must be valid", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusExecNodes")
		execNodes := obj.(*ytv1.YtsaurusExecNodes)
		execNodes.Spec.Sidecars = []string{"invalid"}
		execNodes.Spec.InitContainers = []string{"name: init", "name: init"}
		validator := componentTestValidator(t, obj, parent)
		requireComponentError(t, validator, obj, "YtsaurusExecNodes", "spec.sidecars[0]")
		requireComponentError(t, validator, obj, "YtsaurusExecNodes", "spec.initContainers[1].name")
	})
	t.Run("HTTP proxies inherit HTTPS requirements", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
		parent.Spec.ClusterFeatures = &ytv1.ClusterFeatures{HTTPProxyHaveHTTPSAddress: true}
		requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusHTTPProxies", "spec.transport.httpsSecret")
		obj.(*ytv1.YtsaurusHTTPProxies).Spec.Transport.HTTPSSecret = &corev1.LocalObjectReference{Name: "https"}
		parent.Spec.HTTPProxies[0].Transport.HTTPSSecret = &corev1.LocalObjectReference{Name: "https"}
		_, err := componentTestValidator(t, obj, parent).ValidateCreate(t.Context(), obj)
		require.NoError(t, err)
	})
	t.Run("RPC proxies need a certificate for TLS", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusRPCProxies")
		obj.(*ytv1.YtsaurusRPCProxies).Spec.Transport.TLSRequired = true
		requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusRPCProxies", "spec.transport.tlsSecret")
	})
	t.Run("secure clusters reject insecure component transports", func(t *testing.T) {
		parent, obj, instance := componentTestObjects("YtsaurusTabletNodes")
		parent.Spec.ClusterFeatures = &ytv1.ClusterFeatures{SecureClusterTransports: true}
		instance.NativeTransport = &ytv1.RPCTransportSpec{TLSInsecure: true}
		requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusTabletNodes", "spec.nativeTransport.tlsInsecure")
	})
	t.Run("secure clusters require HTTPS-only proxies", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
		parent.Spec.ClusterFeatures = &ytv1.ClusterFeatures{SecureClusterTransports: true}
		requireComponentError(t, componentTestValidator(t, obj, parent), obj, "YtsaurusHTTPProxies", "spec.transport.disableHttp")
	})
	t.Run("secure clusters require secure RPC proxies", func(t *testing.T) {
		parent, obj, _ := componentTestObjects("YtsaurusRPCProxies")
		parent.Spec.ClusterFeatures = &ytv1.ClusterFeatures{SecureClusterTransports: true}
		validator := componentTestValidator(t, obj, parent)
		requireComponentError(t, validator, obj, "YtsaurusRPCProxies", "spec.transport.tlsRequired")
		requireComponentError(t, validator, obj, "YtsaurusRPCProxies", "spec.transport.tlsSecret")
	})
}
