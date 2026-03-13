package ytconfig

type UIAuthenticationType string

const (
	uiAuthenticationBasic UIAuthenticationType = "basic"
)

type UIPrimaryMaster struct {
	CellTag uint16 `yson:"cellTag"`
}

type UIClusterURLs struct {
	Icon    string  `yson:"icon"`
	Icon2x  string  `yson:"icon2x"`
	IconBig *string `yson:"iconbig,omitempty"`
}

type UICluster struct {
	ID             string               `yson:"id"`
	Name           string               `yson:"name"`
	Proxy          string               `yson:"proxy"`
	ExternalProxy  *string              `yson:"externalProxy,omitempty"`
	Secure         bool                 `yson:"secure"`
	Authentication UIAuthenticationType `yson:"authentication"`
	Group          string               `yson:"group"`
	Theme          string               `yson:"theme"`
	Environment    string               `yson:"environment"`
	Description    string               `yson:"description"`
	PrimaryMaster  UIPrimaryMaster      `yson:"primaryMaster"`
	URLs           *UIClusterURLs       `yson:"urls,omitempty"`
}

type UIClusters struct {
	Clusters []UICluster `yson:"clusters"`
}

func getUIClusterCarcass() UICluster {
	return UICluster{
		Secure:         false,
		Authentication: uiAuthenticationBasic,
		Group:          "My YTsaurus clusters",
		Description:    "My first YTsaurus. Handle with care.",
	}
}

type UICustomSettings struct {
	DirectDownload *bool `yson:"directDownload,omitempty"`
}

type UICustom struct {
	OdinBaseUrl *string           `yson:"odinBaseUrl,omitempty"`
	Settings    *UICustomSettings `yson:"uiSettings,omitempty"`
}
