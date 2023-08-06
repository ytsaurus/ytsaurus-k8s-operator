package resources

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	labeller "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"go.ytsaurus.tech/yt/go/guid"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Job struct {
	name       string
	uniqueName string
	l          *labeller.Labeller
	apiProxy   apiproxy.APIProxy

	oldObject batchv1.Job
	newObject batchv1.Job
}

func NewJob(name string, l *labeller.Labeller, apiProxy apiproxy.APIProxy) *Job {
	return &Job{
		name:       name,
		uniqueName: fmt.Sprintf("%s-%s", name, guid.New().String()),
		l:          l,
		apiProxy:   apiProxy,
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
	j.newObject.ObjectMeta.Name = j.uniqueName
	j.newObject.Spec = batchv1.JobSpec{
		TTLSecondsAfterFinished: &ttlSeconds,
	}

	return &j.newObject
}

func (j *Job) Fetch(ctx context.Context) error {
	jobs := batchv1.JobList{}
	err := j.apiProxy.ListObjects(ctx, &jobs, j.l.GetListOptions(&j.name)...)
	if err != nil {
		return err
	}

	lastCreationTimestamp := v1.Time{}
	var lastJob *batchv1.Job

	for _, job := range jobs.Items {
		if lastCreationTimestamp.Before(&job.CreationTimestamp) {
			lastCreationTimestamp = job.CreationTimestamp
			lastJob = &job
		}
	}
	if lastJob != nil {
		j.oldObject = *lastJob
	}
	return err
}
