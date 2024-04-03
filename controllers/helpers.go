package controllers

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

const (
	defaultClusterDomain = "cluster.local"
)

func getClusterDomain(client client.Client) string {
	domain, exists := os.LookupEnv("K8S_CLUSTER_DOMAIN")
	if exists {
		return domain
	}
	apiSvc := "kubernetes.default.svc"

	cname, err := net.LookupCNAME(apiSvc)
	if err != nil {
		return defaultClusterDomain
	}

	clusterDomain := strings.TrimPrefix(cname, apiSvc)
	clusterDomain = strings.TrimPrefix(clusterDomain, ".")
	clusterDomain = strings.TrimSuffix(clusterDomain, ".")

	return clusterDomain
}

func logComponentStatuses(
	ctx context.Context,
	registry *componentRegistry,
	statuses map[string]components.ComponentStatus,
	componentsOrder [][]consts.ComponentType,
	resource *ytv1.Ytsaurus,
) error {
	logger := log.FromContext(ctx)

	var readyComponents []string
	var notReadyComponents []string

	masterBuildStatus, err := getStatusForMasterBuild(ctx, registry.master)
	if err != nil {
		return err
	}
	logger.V(1).Info(
		fmt.Sprintf(
			"%d.%s %s: %s",
			0,
			statusToSymbol(masterBuildStatus.SyncStatus),
			registry.master.GetName()+" Build",
			masterBuildStatus.Message,
		),
	)

	for batchIndex := 1; batchIndex <= len(componentsOrder); batchIndex++ {
		typesInBatch := componentsOrder[batchIndex-1]
		compsInBatch := registry.listByType(typesInBatch...)
		for _, comp := range compsInBatch {
			name := comp.GetName()
			status := statuses[name]

			if status.SyncStatus == components.SyncStatusReady {
				readyComponents = append(readyComponents, name)
			} else {
				notReadyComponents = append(notReadyComponents, name)
			}

			logger.V(1).Info(
				fmt.Sprintf(
					"%d.%s %s: %s",
					batchIndex,
					statusToSymbol(status.SyncStatus),
					name,
					status.Message,
				),
			)
		}
	}

	// NB: This log is mentioned at https://ytsaurus.tech/docs/ru/admin-guide/install-ytsaurus
	logger.Info("Ytsaurus sync status",
		"notReadyComponents", notReadyComponents,
		"readyComponents", readyComponents,
		"updateState", resource.Status.UpdateStatus.State,
		"clusterState", resource.Status.State)
	return nil
}

func statusToSymbol(st components.SyncStatus) string {
	switch st {
	case components.SyncStatusReady:
		return "[v]"
	case components.SyncStatusBlocked:
		return "[x]"
	case components.SyncStatusUpdating:
		return "[.]"
	default:
		return "[ ]"
	}
}
