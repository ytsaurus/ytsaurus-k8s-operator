package ytconfig

import (
	"fmt"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/yson"
)

type Strawberry struct {
	Root          string `yson:"root"`
	Stage         string `yson:"stage"`
	RobotUsername string `yson:"robot_username"`
}

type StrawberryController struct {
	LocationProxies     []string                 `yson:"location_proxies"`
	Strawberry          Strawberry               `yson:"strawberry"`
	Controllers         map[string]yson.RawValue `yson:"controllers"`
	HTTPAPIEndpoint     string                   `yson:"http_api_endpoint"`
	HTTPLocationAliases map[string][]string      `yson:"http_location_aliases"`
}

type ChytInitCluster struct {
	Proxy          string   `yson:"proxy"`
	StrawberryRoot string   `yson:"strawberry_root"`
	Families       []string `yson:"families"`
}

func getStrawberryController() StrawberryController {
	return StrawberryController{
		Strawberry: Strawberry{
			Root:  "//sys/strawberry",
			Stage: "production",

			RobotUsername: consts.StrawberryControllerUserName,
		},
		Controllers: map[string]yson.RawValue{
			"chyt": yson.RawValue("{address_resolver={enable_ipv4=%true;enable_ipv6=%true;retries=1000}}"),
		},
		HTTPAPIEndpoint: fmt.Sprintf(":%v", consts.StrawberryHTTPAPIPort),
	}
}

func getChytInitCluster() ChytInitCluster {
	return ChytInitCluster{
		StrawberryRoot: "//sys/strawberry",
		Families:       []string{"chyt", "jupyt"},
	}
}
