package consts

const DefaultAdminLogin = "admin"
const DefaultAdminPassword = "password"

const AdminLoginSecret = "login"
const AdminPasswordSecret = "password"
const AdminTokenSecret = "token"

const (
	SuperusersGroupName = "superusers"

	// Superuser does not need password to issue tokens.
	YtsaurusOperatorUserIsSuperuser = true

	YtsaurusOperatorUserName         = "robot-ytsaurus-k8s-operator"
	TimbertruckUserName              = "robot-timbertruck"
	HydraPersistenceUploaderUserName = "robot-hydra-persistence-uploader"
	UIUserName                       = "robot-ui"
	StrawberryControllerUserName     = "robot-strawberry-controller"
	OperationArchivariusUserName     = "operation_archivarius"
	QueueAgentUserName               = "queue_agent"
	QueryTrackerUserName             = "query_tracker"
	YqlAgentUserName                 = "yql_agent"
	ChytReleaserUserName             = "chyt_releaser"
	SpytReleaserUserName             = "spyt_releaser"
)

const DefaultCABundlePath = "/etc/ssl/certs/ca-certificates.crt"
const DefaultYqlTokenPath = "/usr/yql_agent_token"

const StartUID = 19500

const DefaultHTTPProxyRole = "default"
const DefaultName = "default"
const DefaultMedium = "default"

const MaxSlotLocationReserve = 10 << 30 // 10GiB

const DefaultStrawberryControllerFamily = "chyt"

func GetDefaultStrawberryControllerFamilies() []string {
	return []string{"chyt", "jupyt"}
}

const DefaultTimbertruckDirectoryPath = "//sys/admin/logs"
