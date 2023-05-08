package components

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	"strings"
)

type YtsaurusClient interface {
	Component
	GetYtClient() yt.Client
}

type ytsaurusClient struct {
	ComponentBase
	httpProxy Component

	initUserJob *InitJob

	secret   *resources.StringSecret
	ytClient yt.Client
}

func NewYtsaurusClient(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	httpProxy Component,
) YtsaurusClient {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelClient,
		ComponentName:  "YTsaurusClient",
	}

	return &ytsaurusClient{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			apiProxy: apiProxy,
			cfgen:    cfgen,
		},
		httpProxy: httpProxy,
		initUserJob: NewInitJob(
			&labeller,
			apiProxy,
			"user",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			labeller.GetSecretName(),
			&labeller,
			apiProxy),
	}
}

func (yc *ytsaurusClient) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		yc.secret,
		yc.initUserJob,
	})
}

func (yc *ytsaurusClient) createInitUserScript() string {
	token, _ := yc.secret.GetValue(consts.TokenSecretKey)
	initJob := initJobWithNativeDriverPrologue()
	return initJob + "\n" + strings.Join(createUserCommand(consts.YtsaurusOperatorUserName, "", token, true), "\n")
}

func (yc *ytsaurusClient) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if !(yc.httpProxy.Status(ctx) == SyncStatusReady) {
		return SyncStatusBlocked, err
	}

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := yc.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = yc.secret.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !dry {
		yc.initUserJob.SetInitScript(yc.createInitUserScript())
	}
	status, err := yc.initUserJob.Sync(ctx, dry)
	if err != nil || status != SyncStatusReady {
		return status, err
	}

	if yc.ytClient == nil {
		token, _ := yc.secret.GetValue(consts.TokenSecretKey)
		yc.ytClient, err = ythttp.NewClient(&yt.Config{
			Proxy: yc.cfgen.GetHTTPProxiesAddress(),
			Token: token,
		})

		if err != nil {
			return SyncStatusPending, err
		}
	}

	return SyncStatusReady, err
}

func (yc *ytsaurusClient) Status(ctx context.Context) SyncStatus {
	status, err := yc.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (yc *ytsaurusClient) Sync(ctx context.Context) error {
	_, err := yc.doSync(ctx, false)
	return err
}

func (yc *ytsaurusClient) GetYtClient() yt.Client {
	return yc.ytClient
}
