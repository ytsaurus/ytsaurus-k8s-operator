package components

import (
	"context"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	ytv1 "k8s.io/api/core/v1"
)

type bundleControllerInstanceAnnotations struct {
	Allocated            bool   `yson:"allocated"`
	AllocatedForBundle   string `yson:"allocated_for_bundle"`
	DataCenter           string `yson:"data_center"`
	DeallocationStrategy string `yson:"deallocation_strategy"`
	NannyService         string `yson:"nanny_service"`
	YpCluster            string `yson:"yp_cluster"`

	Resources instanceResources `yson:"resources"`
}

type instanceResources struct {
	Type   string `yson:"type"`
	Memory int64  `yson:"memory"`
	Vcpu   int64  `yson:"vcpu"`
}

func getInstanceResources(name string, resources *ytv1.ResourceList) (instanceResources, bool) {
	dummyResult := instanceResources{
		Vcpu:   18000,
		Memory: 120 * 1024 * 1024 * 1024,
	}

	if resources == nil {
		return dummyResult, false
	}

	var result instanceResources
	var exists bool

	result.Type = name
	if result.Vcpu, exists = resources.Cpu().AsInt64(); !exists || result.Vcpu <= 0 {
		return dummyResult, false
	}
	if result.Memory, exists = resources.Memory().AsInt64(); !exists || result.Memory <= 0 {
		return dummyResult, false
	}

	return result, true
}

func getBundleControllerInstanceAnnotations(ctx context.Context, spareBundleName string, resources instanceResources) bundleControllerInstanceAnnotations {
	return bundleControllerInstanceAnnotations{
		Allocated:            true,
		AllocatedForBundle:   spareBundleName,
		DataCenter:           "default",
		DeallocationStrategy: "",
		NannyService:         "undefined",
		YpCluster:            "undefined",
		Resources:            resources,
	}
}

func initBundleControllerAnnotatios(ctx context.Context, dry bool, ytClient yt.Client, instancePath ypath.Path, resources instanceResources) (ComponentStatus, error) {
	var instances []string
	err := ytClient.ListNode(ctx, instancePath, &instances, nil)
	if err != nil {
		return SimpleStatus(SyncStatusBlocked), err
	}

	initialized := true
	annotations := getBundleControllerInstanceAnnotations(ctx, SpareBundle, resources)

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
