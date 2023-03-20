package ytconfig

type UiAuthenticationType string

const (
	uiAuthenticationBasic UiAuthenticationType = "basic"
)

type UiPrimaryMaster struct {
	CellTag int16 `json:"cellTag"`
}

type UiCluster struct {
	Id             string               `json:"id"`
	Name           string               `json:"name"`
	Proxy          string               `json:"proxy"`
	Secure         bool                 `json:"secure"`
	Authentication UiAuthenticationType `json:"authentication"`
	Group          string               `json:"group"`
	Theme          string               `json:"theme"`
	Environment    string               `json:"environment"`
	Description    string               `json:"description"`
	PrimaryMaster  UiPrimaryMaster      `json:"primaryMaster"`
}

type WebUi struct {
	Clusters []UiCluster `json:"clusters"`
}

func getUiClusterCarcass() UiCluster {
	return UiCluster{
		Secure:         false,
		Authentication: uiAuthenticationBasic,
		Group:          "My Ytsaurus clusters",
		Theme:          "lavander",
		Description:    "My first Ytsaurus. Handle with care.",
		Environment:    "testing",
	}
}
