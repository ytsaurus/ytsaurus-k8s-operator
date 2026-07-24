package controllers

import (
	"context"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

const (
	remoteClusterSpecField = "remoteClusterSpec"
)

func GuessClusterDomain(ctx context.Context) (string, error) {
	apiSvc := "kubernetes.default.svc"
	cname, err := net.DefaultResolver.LookupCNAME(ctx, apiSvc)
	if err != nil {
		return "", err
	}

	clusterDomain := strings.TrimPrefix(cname, apiSvc)
	clusterDomain = strings.TrimPrefix(clusterDomain, ".")
	clusterDomain = strings.TrimSuffix(clusterDomain, ".")

	return clusterDomain, nil
}

type BaseReconciler struct {
	client.Client

	APIReader client.Reader // Direct API-server uncached reader

	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	WatchOperatorInstance string
	ClusterDomain         string
}

func (r *BaseReconciler) ShouldIgnoreResource(ctx context.Context, object client.Object) bool {
	// Redundant check and prevention for possible misconfiguration.
	// Normally resources should be filtered by controller runtime cache management.
	// Ignore resources with operator instance label even when started without instance scope.
	if instance := object.GetLabels()[consts.YTOperatorInstanceLabelName]; instance != r.WatchOperatorInstance {
		logger := log.FromContext(ctx)
		logger.Info(
			"Resource is managed by other operator instance",
			"resourceInstance", instance,
			"operatorInstance", r.WatchOperatorInstance,
		)
		return true
	}
	return false
}

// TryGetActual returns true if cached object matches actual version.
// This allows reconciler consistently read own writes into resource status.
func (r *BaseReconciler) TryGetActual(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) (bool, error) {
	if err := r.Get(ctx, key, obj, opts...); err != nil {
		return false, err
	}
	// TODO: Reconciler can cache non-actual version and skip this check until next refresh.
	metadata := &metav1.PartialObjectMetadata{}
	metadata.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	if err := r.APIReader.Get(ctx, key, metadata); err != nil {
		return false, err
	}
	return metadata.GetResourceVersion() == obj.GetResourceVersion(), nil
}
