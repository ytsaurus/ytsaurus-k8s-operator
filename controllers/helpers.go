package controllers

import (
	"context"
	"net"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultClusterDomain   = "cluster.local"
	remoteClusterSpecField = "remoteClusterSpec"
)

func getClusterDomain(ctx context.Context, client client.Client) string {
	domain, exists := os.LookupEnv("K8S_CLUSTER_DOMAIN")
	if exists {
		return domain
	}
	apiSvc := "kubernetes.default.svc"

	cname, err := net.DefaultResolver.LookupCNAME(ctx, apiSvc)
	if err != nil {
		return defaultClusterDomain
	}

	clusterDomain := strings.TrimPrefix(cname, apiSvc)
	clusterDomain = strings.TrimPrefix(clusterDomain, ".")
	clusterDomain = strings.TrimSuffix(clusterDomain, ".")

	return clusterDomain
}
