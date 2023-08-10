package apiproxy

import (
	"context"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Spyt struct {
	apiProxy APIProxy
	spyt     *ytv1.Spyt
}

func NewSpyt(
	spyt *ytv1.Spyt,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *Spyt {
	return &Spyt{
		spyt:     spyt,
		apiProxy: NewAPIProxy(spyt, client, recorder, scheme),
	}
}

func (c *Spyt) GetResource() *ytv1.Spyt {
	return c.spyt
}

func (c *Spyt) APIProxy() APIProxy {
	return c.apiProxy
}

func (c *Spyt) SetStatusCondition(ctx context.Context, condition metav1.Condition) error {
	meta.SetStatusCondition(&c.spyt.Status.Conditions, condition)
	return c.apiProxy.UpdateStatus(ctx)
}

func (c *Spyt) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.spyt.Status.Conditions, conditionType)
}

func (c *Spyt) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.spyt.Status.Conditions, conditionType)
}
