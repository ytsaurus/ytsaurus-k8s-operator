package controllers

import (
	"net"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultClusterDomain = "cluster.local"
)

func getClusterDomain(client client.Client) string {
	domain, exists := os.LookupEnv("K8S_CLUSTER_DOMAIN")
	if exists {
		return domain
	}
	apiSvc := "kubernetes.default.svc"

	cname, err := net.LookupCNAME(apiSvc)
	if err != nil {
		return DefaultClusterDomain
	}

	clusterDomain := strings.TrimPrefix(cname, apiSvc)
	clusterDomain = strings.TrimPrefix(clusterDomain, ".")
	clusterDomain = strings.TrimSuffix(clusterDomain, ".")

	return clusterDomain
}
