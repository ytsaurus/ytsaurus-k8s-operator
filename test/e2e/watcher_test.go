package controllers_test

import (
	"context"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"

	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

type NamespaceWatcher struct {
	kubeWatcher watch.Interface
	stopCh      chan struct{}
	events      []watch.Event
}

func NewNamespaceWatcher(ctx context.Context, namespace string) *NamespaceWatcher {
	watcher, err := testutil.NewCombinedKubeWatcher(ctx, k8sClient, namespace, []client.ObjectList{
		&batchv1.JobList{},
	})
	Expect(err).ToNot(HaveOccurred())
	return &NamespaceWatcher{
		kubeWatcher: watcher,
		stopCh:      make(chan struct{}),
	}
}

func (w *NamespaceWatcher) Start() {
	go w.loop()
}

func (w *NamespaceWatcher) Stop() {
	close(w.stopCh)
	w.kubeWatcher.Stop()
}

func (w *NamespaceWatcher) GetRawEvents() []watch.Event {
	return w.events
}

// TODO: not really generic, but good enough for the start.
func (w *NamespaceWatcher) GetCompletedJobNames() []string {
	var result []string
	for _, ev := range w.events {
		if job, ok := ev.Object.(*batchv1.Job); ok {
			if job.Status.Succeeded == 0 {
				continue
			}
			result = append(result, job.Name)
		}
	}
	return result
}

func (w *NamespaceWatcher) loop() {
	for {
		select {
		case <-w.stopCh:
			return
		case ev := <-w.kubeWatcher.ResultChan():
			w.events = append(w.events, ev)
		}
	}
}
