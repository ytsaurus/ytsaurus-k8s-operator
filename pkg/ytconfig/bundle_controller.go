package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type BundleController struct {
	Cluster                      string `yson:"cluster"`
	RootPath                     string `yson:"root_path"`
	BundleScanPeriod             int    `yson:"bundle_scan_period"`
	HulkAllocationsPath          string `yson:"hulk_allocations_path"`
	HulkAllocationsHistoryPath   string `yson:"hulk_allocations_history_path"`
	HulkDeallocationsPath        string `yson:"hulk_deallocations_path"`
	HulkDeallocationsHistoryPath string `yson:"hulk_deallocations_history_path"`
	HasInstanceAllocatorService  bool   `yson:"has_instance_allocator_service"`
	EnableSpareNodeAssignment    bool   `yson:"enable_spare_node_assignment"`
}

type ElectionManager struct {
	LockPath string `yson:"lock_path"`
}

type BundleControllerServer struct {
	CommonServer

	EnableBundleController bool `yson:"enable_bundle_controller"`
	EnableCellBalancer     bool `yson:"enable_cell_balancer"`

	BundleController BundleController `yson:"bundle_controller"`
	ElectionManager  ElectionManager  `yson:"election_manager"`
}

func getBundleControllerLogging(spec *ytv1.BundleControllerSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.BundleControllerType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getBundleControllerServerCarcass(spec *ytv1.BundleControllerSpec) (BundleControllerServer, error) {
	var c BundleControllerServer

	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.BundleControllerMonitoringPort)
	c.RPCPort = consts.BundleControllerRPCPort

	c.Logging = getBundleControllerLogging(spec)

	c.BundleController = BundleController{
		RootPath:                     "//sys/bundle_controller/controller",
		BundleScanPeriod:             1000,
		HulkAllocationsPath:          "//sys/bundle_controller/internal_allocations/allocation_requests",
		HulkAllocationsHistoryPath:   "//sys/bundle_controller/internal_allocations/allocation_requests_history",
		HulkDeallocationsPath:        "//sys/bundle_controller/internal_allocations/deallocation_requests",
		HulkDeallocationsHistoryPath: "//sys/bundle_controller/internal_allocations/deallocation_requests_history",
		HasInstanceAllocatorService:  false,
		EnableSpareNodeAssignment:    false,
	}

	c.ElectionManager = ElectionManager{
		LockPath: "//sys/bundle_controller/coordinator/lock",
	}

	c.EnableBundleController = true
	c.EnableCellBalancer = false

	return c, nil
}
