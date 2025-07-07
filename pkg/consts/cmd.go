package consts

import (
	"time"
)

const (
	ConfigMountPoint           = "/config"
	ConfigTemplateMountPoint   = "/config_template"
	HTTPSSecretMountPoint      = "/tls/https_secret"
	RPCProxySecretMountPoint   = "/tls/rpc_secret"
	BusServerSecretMountPoint  = "/tls/bus_secret"
	BusClientSecretMountPoint  = "/tls/bus_client_secret"
	CABundleMountPoint         = "/tls/ca_bundle"
	UIClustersConfigMountPoint = "/opt/app"
	UICustomConfigMountPoint   = "/opt/app/dist/server/configs/custom"
	UISecretsMountPoint        = "/opt/app/secrets"
	UIVaultMountPoint          = "/vault"
)

const (
	YTServerContainerName                 = "ytserver"
	PostprocessConfigContainerName        = "postprocess-config"
	PrepareLocationsContainerName         = "prepare-locations"
	PrepareSecretContainerName            = "prepare-secret"
	UIContainerName                       = "yt-ui"
	StrawberryContainerName               = "strawberry"
	HydraPersistenceUploaderContainerName = "hydra-persistence-uploader"
	TimbertruckContainerName              = "timbertruck"
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
	JobsContainerName = "jobs"

	ContainerdConfigVolumeName = "config-containerd"
	ContainerdConfigMountPoint = "/config/containerd"
	ContainerdSocketName       = "containerd.sock"
	ContainerdConfigFileName   = "containerd.toml"

	CRINamespace  = "yt"
	CRIBaseCgroup = "/yt"
)

const (
	ConfigTemplateVolumeName = "config-template"
	ConfigVolumeName         = "config"
	HTTPSSecretVolumeName    = "https-secret"
	RPCProxySecretVolumeName  = "rpc-secret"
	BusServerSecretVolumeName = "bus-secret"
	BusClientSecretVolumeName = "bus-client-secret"
	CABundleVolumeName       = "ca-bundle"
	InitScriptVolumeName     = "init-script"
	UIVaultVolumeName        = "vault"
	UISecretsVolumeName      = "secrets"
	TimbertruckWorkDirName   = "timbertruck"
)

const (
	// Pass certificates via secure vault too - to avoid logging bulky values
	BusCABundleVaultName          = "YT_BUS_CA_BUNDLE"
	BusClientCertificateVaultName = "YT_BUS_CLIENT_CERTIFICATE"
	BusClientPrivateKeyVaultName  = "YT_BUS_CLIENT_PRIVATE_KEY"
	BusServerCertificateVaultName = "YT_BUS_SERVER_CERTIFICATE"
	BusServerPrivateKeyVaultName  = "YT_BUS_SERVER_PRIVATE_KEY"

	SecureVaultEnvPrefix = "YT_SECURE_VAULT_"
)

const (
	HTTPSSecretUpdatePeriod = time.Second * 60
)
