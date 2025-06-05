package consts

const DefaultAdminLogin = "admin"
const DefaultAdminPassword = "password"

const AdminLoginSecret = "login"
const AdminPasswordSecret = "password"
const AdminTokenSecret = "token"

const DefaultCABundlePath = "/etc/ssl/certs/ca-certificates.crt"

const HydraPersistenceUploaderUserName = "robot-hydra-persistence-uploader"

const UIUserName = "robot-ui"
const StrawberryControllerUserName = "robot-strawberry-controller"
const YtsaurusOperatorUserName = "robot-ytsaurus-k8s-operator"

const YqlUserName = "yql_agent"
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
