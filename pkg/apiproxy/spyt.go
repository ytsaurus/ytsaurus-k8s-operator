package apiproxy

import (
	"context"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Spyt struct {
	APIProxy

	spyt *ytv1.Spyt
}

func NewSpyt(
	spyt *ytv1.Spyt,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme,
) *Spyt {
	return &Spyt{
		APIProxy: NewAPIProxy(spyt, client, recorder, scheme),
		spyt:     spyt,
	}
}

func (c *Spyt) GetResource() *ytv1.Spyt {
	return c.spyt
}

func (c *Spyt) SaveReleaseStatus(ctx context.Context, releaseStatus ytv1.SpytReleaseStatus) error {
	logger := log.FromContext(ctx)
	c.GetResource().Status.ReleaseStatus = releaseStatus
	if err := c.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Spyt release status")
		return err
	}

	return nil
}
