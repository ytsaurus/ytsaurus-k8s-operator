package consts

const (
	ConfigMountPoint           = "/config"
	HTTPSSecretMountPoint      = "/config/https_secret"
	UIClustersConfigMountPoint = "/opt/app"
	UICustomConfigMountPoint   = "/opt/app/dist/server/configs/custom"
	UISecretsMountPoint        = "/opt/app/secrets"
	UIVaultMountPoint          = "/vault"
)

const (
	YTServerContainerName         = "ytserver"
	PrepareLocationsContainerName = "prepare-locations"
	PrepareSecretContainerName    = "prepare-secret"
	UIContainerName               = "yt-ui"
)

const (
	ClientConfigFileName = "client.yson"

	InitClusterScriptFileName = "init-cluster.sh"

	UIClusterConfigFileName = "clusters-config.json"
	UISecretFileName        = "yt-interface-secret.json"
	TokenSecretKey          = "YT_TOKEN"
)

const (
	ConfigVolumeName      = "config"
	HTTPSSecretVolumeName = "https-secret"
	InitScriptVolumeName  = "init-script"
	UIVaultVolumeName     = "vault"
	UISecretsVolumeName   = "secrets"
)
