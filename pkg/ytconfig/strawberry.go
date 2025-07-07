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
	LocationProxies        []string                 `yson:"location_proxies"`
	Strawberry             Strawberry               `yson:"strawberry"`
	Controllers            map[string]yson.RawValue `yson:"controllers"`
	HTTPAPIEndpoint        string                   `yson:"http_api_endpoint"`
	HTTPLocationAliases    map[string][]string      `yson:"http_location_aliases"`
	HTTPControllerMappings map[string]string        `yson:"http_controller_mappings"`
}

type StrawberryInitCluster struct {
	Proxy          string   `yson:"proxy"`
	StrawberryRoot string   `yson:"strawberry_root"`
	Families       []string `yson:"families"`
}

type ChytConfig struct {
	AddressResolver  AddressResolver   `yson:"address_resolver"`
	BusServer        *BusServer        `yson:"bus_server,omitempty"`
	SecureVaultFiles map[string]string `yson:"secure_vault_files,omitempty"`
}

func getStrawberryController(
	conFamConfig StrawberryControllerFamiliesConfig,
	resolver AddressResolver,
	busServer *BusServer,
	keyring *Keyring,
) (StrawberryController, error) {
	controllers := make(map[string]yson.RawValue, len(conFamConfig.ControllerFamilies))
	for _, cFamily := range conFamConfig.ControllerFamilies {
		var config any

		switch cFamily {
		case "chyt":
			vault := map[string]string{}
			if keyring.BusCABundle != nil && keyring.BusCABundle.FileName != consts.DefaultCABundlePath {
				vault[consts.BusCABundleVaultName] = keyring.BusCABundle.FileName
			}
			if keyring.BusClientCertificate != nil {
				vault[consts.BusClientCertificateVaultName] = keyring.BusClientCertificate.FileName
			}
			if keyring.BusClientPrivateKey != nil {
				vault[consts.BusClientPrivateKeyVaultName] = keyring.BusClientPrivateKey.FileName
			}
			if keyring.BusServerCertificate != nil {
				vault[consts.BusServerCertificateVaultName] = keyring.BusServerCertificate.FileName
			}
			if keyring.BusServerPrivateKey != nil {
				vault[consts.BusServerPrivateKeyVaultName] = keyring.BusServerPrivateKey.FileName
			}
			config = ChytConfig{
				AddressResolver:  resolver,
				BusServer:        busServer,
				SecureVaultFiles: vault,
			}
		default:
			config = struct{}{}
		}

		ysonConfig, err := marshallYsonConfig(config)
		if err != nil {
			return StrawberryController{}, err
		}
		controllers[cFamily] = ysonConfig
	}

	var httpControllerMappings map[string]string

	if conFamConfig.ExternalProxy != nil {
		httpControllerMappings = make(map[string]string, len(conFamConfig.ControllerFamilies))
		for _, cFamily := range conFamConfig.ControllerFamilies {
			if cFamily == conFamConfig.DefaultRouteFamily {
				httpControllerMappings["*"] = cFamily
			} else {
				host := fmt.Sprintf("%s.%s", cFamily, *conFamConfig.ExternalProxy)
				httpControllerMappings[host] = cFamily
			}
		}
	} else {
		httpControllerMappings = map[string]string{"*": conFamConfig.DefaultRouteFamily}
	}

	return StrawberryController{
		Strawberry: Strawberry{
			Root:  "//sys/strawberry",
			Stage: "production",

			RobotUsername: consts.StrawberryControllerUserName,
		},
		Controllers:            controllers,
		HTTPAPIEndpoint:        fmt.Sprintf(":%v", consts.StrawberryHTTPAPIPort),
		HTTPControllerMappings: httpControllerMappings,
	}, nil
}

func getStrawberryInitCluster(conFamConfig StrawberryControllerFamiliesConfig) StrawberryInitCluster {
	return StrawberryInitCluster{
		StrawberryRoot: "//sys/strawberry",
		Families:       conFamConfig.ControllerFamilies,
	}
}
