package apiproxy

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	meta.SetStatusCondition(&c.chyt.Status.Conditions, condition)
}

func (c *Chyt) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.chyt.Status.Conditions, conditionType)
}

func (c *Chyt) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.chyt.Status.Conditions, conditionType)
}
