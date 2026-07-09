package consts

import (
	"time"
)

const (
	// YTsaurus tokens follow format "ytct-{4}-{32}".
	TokenPrefixPrefix = "ytct-"

	// Token prefix "ytct-XXXX-" which is safe to disclose.
	TokenPrefixLength  = 10
	TokenMinimalLength = 40

	// Hashed token prefix "XXXXXXXX" which is safe to disclose.
	TokenHashPrefixLength  = 8
	TokenMinimalHashLength = 64
)

const SuperusersGroupName = "superusers"

const DefaultAdminLogin = "admin"
const DefaultAdminPassword = "password"

const RandomTokenPrefix = "ytct-ytop-"
const RandomTokenBytes = 30

const AdminLoginSecretKey = "login"
const AdminPasswordSecretKey = "password"
const AdminTokenSecretKey = "token"

const (
	YtsaurusOperatorUserName         = "robot-ytsaurus-k8s-operator"
	TimbertruckUserName              = "robot-timbertruck"
	HydraPersistenceUploaderUserName = "robot-hydra-persistence-uploader"
	UIUserName                       = "robot-ui"
	StrawberryControllerUserName     = "robot-strawberry-controller"
	OperationArchivariusUserName     = "operation_archivarius"
	QueueAgentUserName               = "queue_agent"
	QueryTrackerUserName             = "query_tracker"
	YQLAgentUserName                 = "yql_agent"
	YQLAgentExecUserName             = "yql_agent_exec"
	ChytReleaserUserName             = "chyt_releaser"
	SpytReleaserUserName             = "spyt_releaser"
)

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

const DefaultImageHeaterConcurrency = 100

const DefaultClusterStatusPollPeriod = time.Minute
