package consts

import (
	"time"
)

const (
	ConfigMountPoint           = "/config"
	ConfigTemplateMountPoint   = "/config_template"
	HTTPSSecretMountPoint      = "/config/https_secret"
	RPCSecretMountPoint        = "/config/rpc_secret"
	BusSecretMountPoint        = "/config/bus_secret"
	CABundleMountPoint         = "/config/ca_bundle"
	UIClustersConfigMountPoint = "/opt/app"
	UICustomConfigMountPoint   = "/opt/app/dist/server/configs/custom"
	UISecretsMountPoint        = "/opt/app/secrets"
	UIVaultMountPoint          = "/vault"
)

const (
	YTServerContainerName          = "ytserver"
	PostprocessConfigContainerName = "postprocess-config"
	PrepareLocationsContainerName  = "prepare-locations"
	PrepareSecretContainerName     = "prepare-secret"
	UIContainerName                = "yt-ui"
)

const (
	ClientConfigFileName = "client.yson"

	InitClusterScriptFileName       = "init-cluster.sh"
	PostprocessConfigScriptFileName = "postprocess-config.sh"

	UIClusterConfigFileName = "clusters-config.json"
	UISecretFileName        = "yt-interface-secret.json"
	CABundleFileName        = "ca.crt"
	TokenSecretKey          = "YT_TOKEN"
)

const (
	ConfigTemplateVolumeName = "config-template"
	ConfigVolumeName         = "config"
	HTTPSSecretVolumeName    = "https-secret"
	RPCSecretVolumeName      = "rpc-secret"
	BusSecretVolumeName      = "bus-secret"
	CABundleVolumeName       = "ca-bundle"
	InitScriptVolumeName     = "init-script"
	UIVaultVolumeName        = "vault"
	UISecretsVolumeName      = "secrets"
)

const (
	HTTPSSecretUpdatePeriod = time.Second * 60
)
