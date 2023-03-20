package consts

const (
	ConfigMountPoint    = "/config"
	UiConfigMountPoint  = "/opt/app/"
	UiSecretsMountPoint = "/opt/app/secrets"
	UiVaultMountPoint   = "/vault"
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
	ConfigVolumeName     = "config"
	InitScriptVolumeName = "init-script"
	UiVaultVolumeName    = "vault"
	UiSecretsVolumeName  = "secrets"
)
