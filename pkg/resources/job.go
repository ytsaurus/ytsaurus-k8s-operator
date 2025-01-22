package resources

import (
	batchv1 "k8s.io/api/batch/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type Job struct {
	BaseManagedResource[*batchv1.Job]
}

func NewJob(name string, l *labeller.Labeller, apiProxy apiproxy.APIProxy) *Job {
	return &Job{
		BaseManagedResource: BaseManagedResource[*batchv1.Job]{
			proxy:     apiProxy,
			labeller:  l,
			name:      name,
			oldObject: &batchv1.Job{},
			newObject: &batchv1.Job{},
		},
	}
}

func (j *Job) Completed() bool {
	return j.oldObject.Status.Succeeded > 0
}

func (j *Job) Build() *batchv1.Job {
	var ttlSeconds int32 = 600
	var backoffLimit int32 = 15
	j.newObject.ObjectMeta = j.labeller.GetObjectMeta(j.name)
	j.newObject.Spec = batchv1.JobSpec{
		TTLSecondsAfterFinished: &ttlSeconds,
		BackoffLimit:            &backoffLimit,
	}

	return j.newObject
}
