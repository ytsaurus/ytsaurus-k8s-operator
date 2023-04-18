package ytconfig

type UIAuthenticationType string

const (
	uiAuthenticationBasic UIAuthenticationType = "basic"
)

type UIPrimaryMaster struct {
	CellTag int16 `json:"cellTag"`
}

type UICluster struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Proxy          string               `json:"proxy"`
	Secure         bool                 `json:"secure"`
	Authentication UIAuthenticationType `json:"authentication"`
	Group          string               `json:"group"`
	Theme          string               `json:"theme"`
	Environment    string               `json:"environment"`
	Description    string               `json:"description"`
	PrimaryMaster  UIPrimaryMaster      `json:"primaryMaster"`
}

type WebUI struct {
	Clusters []UICluster `json:"clusters"`
}

func getUIClusterCarcass() UICluster {
	return UICluster{
		Secure:         false,
		Authentication: uiAuthenticationBasic,
		Group:          "My Ytsaurus clusters",
		Theme:          "lavander",
		Description:    "My first Ytsaurus. Handle with care.",
		Environment:    "testing",
	}
}
