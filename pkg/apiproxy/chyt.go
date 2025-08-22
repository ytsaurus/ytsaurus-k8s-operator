package apiproxy

import (
	"context"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Chyt struct {
	apiProxy APIProxy
	chyt     *ytv1.Chyt
}

func NewChyt(
	chyt *ytv1.Chyt,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *Chyt {
	return &Chyt{
		chyt:     chyt,
		apiProxy: NewAPIProxy(chyt, client, recorder, scheme),
	}
}

func (c *Chyt) GetResource() *ytv1.Chyt {
	return c.chyt
}

func (c *Chyt) APIProxy() APIProxy {
	return c.apiProxy
}

func (c *Chyt) SetStatusCondition(condition metav1.Condition) {
	condition.ObservedGeneration = c.chyt.Generation
	meta.SetStatusCondition(&c.chyt.Status.Conditions, condition)
}

func (c *Chyt) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.chyt.Status.Conditions, conditionType)
}

func (c *Chyt) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.chyt.Status.Conditions, conditionType)
}

func (c *Chyt) SaveReleaseStatus(ctx context.Context, releaseStatus ytv1.ChytReleaseStatus) error {
	logger := log.FromContext(ctx)
	c.GetResource().Status.ReleaseStatus = releaseStatus
	if err := c.apiProxy.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Chyt release status")
		return err
	}

	return nil
}
