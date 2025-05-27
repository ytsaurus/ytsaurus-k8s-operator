package components

import (
	"context"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
)

type bundleControllerInstanceAnnotations struct {
	Allocated            bool   `yson:"allocated"`
	AllocatedForBundle   string `yson:"allocated_for_bundle"`
	DataCenter           string `yson:"data_center"`
	DeallocationStrategy string `yson:"deallocation_strategy"`
	NannyService         string `yson:"nanny_service"`
	YpCluster            string `yson:"yp_cluster"`
}

func getBundleControllerInstanceAnnotations(spareBundleName string) bundleControllerInstanceAnnotations {
	return bundleControllerInstanceAnnotations{
		Allocated:            true,
		AllocatedForBundle:   spareBundleName,
		DataCenter:           "default",
		DeallocationStrategy: "",
		NannyService:         "undefined",
		YpCluster:            "undefined",
	}
}

func initBundleControllerAnnotatios(ctx context.Context, dry bool, ytClient yt.Client, instancePath ypath.Path) (ComponentStatus, error) {
	var instances []string
	err := ytClient.ListNode(ctx, instancePath, &instances, nil)
	if err != nil {
		return SimpleStatus(SyncStatusBlocked), err
	}

	initialized := true
	annotations := getBundleControllerInstanceAnnotations(SpareBundle)

	for _, instance := range instances {
		annotationsPath := instancePath.Child(instance).Attr("bundle_controller_annotations")
		exists, err := ytClient.NodeExists(ctx, annotationsPath, nil)
		if err != nil {
			return SimpleStatus(SyncStatusBlocked), err
		}

		if exists {
			continue
		} else {
			initialized = false
		}

		if dry {
			continue
		}

		err = ytClient.SetNode(ctx, annotationsPath, annotations, nil)
		if err != nil {
			return SimpleStatus(SyncStatusBlocked), err
		}
	}

	if initialized {
		return SimpleStatus(SyncStatusReady), nil
	}

	status := SyncStatusUpdating
	if dry {
		status = SyncStatusPending
	}

	return WaitingStatus(status, "bundle_controller_annotations init"), err
}
