package controllers

import (
	"context"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	OperatorInstance string
	ClusterDomain    string
}
