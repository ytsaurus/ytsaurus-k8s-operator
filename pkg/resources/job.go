package resources

import (
	"slices"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

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

func (j *Job) Status() batchv1.JobStatus {
	return j.oldObject.Status
}

func (j *Job) IsStatusConditionTrue(conditionType batchv1.JobConditionType) bool {
	return slices.ContainsFunc(j.Status().Conditions, func(c batchv1.JobCondition) bool {
		return c.Type == conditionType && c.Status == corev1.ConditionTrue
	})
}

func (j *Job) IsComplete() bool {
	return j.IsStatusConditionTrue(batchv1.JobComplete)
}

func (j *Job) IsFailed() bool {
	return j.IsStatusConditionTrue(batchv1.JobFailed)
}

func (j *Job) Duration() time.Duration {
	status := j.Status()
	if status.StartTime != nil && status.CompletionTime != nil {
		return status.CompletionTime.Sub(status.StartTime.Time)
	}
	return time.Duration(-1)
}

func (j *Job) Build() *batchv1.Job {
	j.newObject.ObjectMeta = j.labeller.GetObjectMeta(j.name)
	j.newObject.Spec = batchv1.JobSpec{}
	return j.newObject
}
