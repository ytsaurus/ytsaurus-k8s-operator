package ytconfig

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/yson"
)

type Strawberry struct {
	Root          string `yson:"root"`
	Stage         string `yson:"stage"`
	RobotUsername string `yson:"robot_username"`
}

type ChytController struct {
	LocationProxies []string      `yson:"location_proxies"`
	Strawberry      Strawberry    `yson:"strawberry"`
	Controller      yson.RawValue `yson:"controller"`
}

type ChytInitCluster struct {
	Proxy          string `yson:"proxy"`
	StrawberryRoot string `yson:"strawberry_root"`
}

func getChytController() ChytController {
	return ChytController{
		Strawberry: Strawberry{
			Root:          "//sys/clickhouse/strawberry",
			Stage:         "production",
			RobotUsername: consts.ChytUserName,
		},
		Controller: yson.RawValue("{address_resolver={enable_ipv4=%true;enable_ipv6=%false;retries=1000}}"),
	}
}

func getChytInitCluster() ChytInitCluster {
	return ChytInitCluster{
		StrawberryRoot: "//sys/clickhouse/strawberry",
	}
}
