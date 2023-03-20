package consts

const YTComponentLabelName = "yt_component"

type YTComponentLabel string

const (
	YTComponentLabelMaster          YTComponentLabel = "yt-master"
	YTComponentLabelScheduler       YTComponentLabel = "yt-scheduler"
	YTComponentLabelСontrollerAgent YTComponentLabel = "yt-controller-agent"
	YTComponentLabelDataNode        YTComponentLabel = "yt-data-node"
	YTComponentLabelExecNode        YTComponentLabel = "yt-exec-node"
	YTComponentLabelTabletNode      YTComponentLabel = "yt-tablet-node"
	YTComponentLabelHttpProxy       YTComponentLabel = "yt-http-proxy"
	YTComponentLabelUi              YTComponentLabel = "yt-ui"
)

func GetYTComponentLabels(value YTComponentLabel) map[string]string {
	return map[string]string{
		YTComponentLabelName: string(value),
	}
}
