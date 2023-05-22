package consts

const YTComponentLabelName = "yt_component"
const YTMetricsLabelName = "yt_metrics"

type YTComponentLabel string

const (
	YTComponentLabelDiscovery       YTComponentLabel = "yt-discovery"
	YTComponentLabelMaster          YTComponentLabel = "yt-master"
	YTComponentLabelScheduler       YTComponentLabel = "yt-scheduler"
	YTComponentLabelControllerAgent YTComponentLabel = "yt-controller-agent"
	YTComponentLabelDataNode        YTComponentLabel = "yt-data-node"
	YTComponentLabelExecNode        YTComponentLabel = "yt-exec-node"
	YTComponentLabelTabletNode      YTComponentLabel = "yt-tablet-node"
	YTComponentLabelHTTPProxy       YTComponentLabel = "yt-http-proxy"
	YTComponentLabelRPCProxy        YTComponentLabel = "yt-rpc-proxy"
	YTComponentLabelUI              YTComponentLabel = "yt-ui"
	YTComponentLabelYqlAgent        YTComponentLabel = "yt-yql-agent"
	YTComponentLabelClient          YTComponentLabel = "yt-client"
)

func GetYTComponentLabels(value YTComponentLabel) map[string]string {
	return map[string]string{
		YTComponentLabelName: string(value),
	}
}
