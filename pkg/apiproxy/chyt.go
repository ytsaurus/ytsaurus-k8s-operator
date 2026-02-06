package apiproxy

import (
	"context"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Chyt struct {
	APIProxy

	chyt *ytv1.Chyt
}

func NewChyt(
	chyt *ytv1.Chyt,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *Chyt {
	return &Chyt{
		APIProxy: NewAPIProxy(chyt, client, recorder, scheme),
		chyt:     chyt,
	}
}

func (c *Chyt) GetResource() *ytv1.Chyt {
	return c.chyt
}

func (c *Chyt) SaveReleaseStatus(ctx context.Context, releaseStatus ytv1.ChytReleaseStatus) error {
	logger := log.FromContext(ctx)
	c.GetResource().Status.ReleaseStatus = releaseStatus
	if err := c.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Chyt release status")
		return err
	}

	return nil
}
