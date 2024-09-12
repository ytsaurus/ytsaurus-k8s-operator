package resources

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type Job struct {
	name         string
	l            *labeller.Labeller
	apiProxy     apiproxy.APIProxy
	tolerations  []corev1.Toleration
	nodeSelector map[string]string

	oldObject batchv1.Job
	newObject batchv1.Job
}

func NewJob(
	name string,
	l *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	tolerations []corev1.Toleration,
	nodeSelector map[string]string,
) *Job {
	return &Job{
		name:         name,
		l:            l,
		apiProxy:     apiProxy,
		tolerations:  tolerations,
		nodeSelector: nodeSelector,
	}
}

func (j *Job) OldObject() client.Object {
	return &j.oldObject
}

func (j *Job) Name() string {
	return j.name
}

func (j *Job) Completed() bool {
	return j.oldObject.Status.Succeeded > 0
}

func (j *Job) Sync(ctx context.Context) error {
	return j.apiProxy.SyncObject(ctx, &j.oldObject, &j.newObject)
}

func (j *Job) Build() *batchv1.Job {
	var ttlSeconds int32 = 600
	var backoffLimit int32 = 15
	j.newObject.ObjectMeta = j.l.GetObjectMeta(j.name)
	j.newObject.Spec = batchv1.JobSpec{
		TTLSecondsAfterFinished: &ttlSeconds,
		BackoffLimit:            &backoffLimit,
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				NodeSelector: j.nodeSelector,
				Tolerations:  j.tolerations,
			},
		},
	}

	return &j.newObject
}

func (j *Job) Fetch(ctx context.Context) error {
	return j.apiProxy.FetchObject(ctx, j.name, &j.oldObject)
}
