package consts

const YTClusterLabelName = "ytsaurus.tech/cluster-name"
const YTComponentLabelName = "yt_component"
const YTMetricsLabelName = "yt_metrics"

// TODO(achulkov2): We should probably use `ytsaurus` instead of `yt` everywhere, but
// it will be an inconvenient change that requires all statefulsets to be recreated.
const (
	YTComponentLabelDiscovery       string = "yt-discovery"
	YTComponentLabelMaster          string = "yt-master"
	YTComponentLabelScheduler       string = "yt-scheduler"
	YTComponentLabelControllerAgent string = "yt-controller-agent"
	YTComponentLabelDataNode        string = "yt-data-node"
	YTComponentLabelExecNode        string = "yt-exec-node"
	YTComponentLabelTabletNode      string = "yt-tablet-node"
	YTComponentLabelHTTPProxy       string = "yt-http-proxy"
	YTComponentLabelRPCProxy        string = "yt-rpc-proxy"
	YTComponentLabelTCPProxy        string = "yt-tcp-proxy"
	YTComponentLabelUI              string = "yt-ui"
	YTComponentLabelYqlAgent        string = "yt-yql-agent"
	YTComponentLabelClient          string = "yt-client"
	YTComponentLabelMasterCache     string = "yt-master-cache"
	YTComponentLabelQueueAgent      string = "yt-queue-agent"
	YTComponentLabelQueryTracker    string = "yt-query-tracker"
	YTComponentLabelChyt            string = "yt-chyt"
	YTComponentLabelSpyt            string = "yt-spyt"
	YTComponentLabelStrawberry      string = "yt-strawberry-controller"
)
