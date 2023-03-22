package labeller

import (
	"fmt"
	"strings"

	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FetchableObject struct {
	Name   string
	Object client.Object
}

type Labeller struct {
	APIProxy       *apiproxy.APIProxy
	Ytsaurus       *ytv1.Ytsaurus
	ComponentLabel consts.YTComponentLabel
	ComponentName  string
}

func (r *Labeller) GetSecretName() string {
	return fmt.Sprintf("%s-secret", r.ComponentLabel)
}

func (r *Labeller) GetMainConfigMapName() string {
	return fmt.Sprintf("%s-config", r.ComponentLabel)
}

func (r *Labeller) GetInitJobName(name string) string {
	return fmt.Sprintf("%s-init-job-%s", r.ComponentLabel, strings.ToLower(name))
}

func (r *Labeller) GetObjectMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: r.Ytsaurus.Namespace,
		Labels:    r.GetMetaLabelMap(),
	}
}

func (r *Labeller) GetYTLabelValue() string {
	return fmt.Sprintf("%s-%s", r.Ytsaurus.Name, r.ComponentLabel)
}

func (r *Labeller) GetSelectorLabelMap() map[string]string {
	return map[string]string{
		consts.YTComponentLabelName: r.GetYTLabelValue(),
	}
}

func (r *Labeller) GetListOptions() []client.ListOption {
	return []client.ListOption{
		client.InNamespace(r.Ytsaurus.Namespace),
		client.MatchingLabels(r.GetSelectorLabelMap()),
	}
}

func (r *Labeller) GetMetaLabelMap() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "Ytsaurus",
		"app.kubernetes.io/instance":   r.Ytsaurus.Name,
		"app.kubernetes.io/component":  string(r.ComponentLabel),
		"app.kubernetes.io/managed-by": "Ytsaurus-k8s-operator",
		consts.YTComponentLabelName:    r.GetYTLabelValue(),
	}
}
