package resources

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Job struct {
	name     string
	l        *labeller.Labeller
	apiProxy apiproxy.APIProxy

	oldObject batchv1.Job
	newObject batchv1.Job
}

func NewJob(name string, l *labeller.Labeller, apiProxy apiproxy.APIProxy) *Job {
	return &Job{
		name:     name,
		l:        l,
		apiProxy: apiProxy,
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
	j.newObject.ObjectMeta = j.l.GetObjectMeta(j.name)
	j.newObject.Spec = batchv1.JobSpec{
		TTLSecondsAfterFinished: &ttlSeconds,
	}

	return &j.newObject
}

func (j *Job) Fetch(ctx context.Context) error {
	return j.apiProxy.FetchObject(ctx, j.name, &j.oldObject)
}
