package controllers

import (
	"context"
	"net"
	"strings"
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
