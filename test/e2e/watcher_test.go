package controllers_test

import (
	"context"

	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

type NamespaceWatcher struct {
	kubeWatcher watch.Interface
	stopCh      chan struct{}
	events      []testEvent
}

type testEvent struct {
	Kind string
	Name string
}

func NewNamespaceWatcher(ctx context.Context, namespace string) *NamespaceWatcher {
	watcher, err := testutil.NewCombinedKubeWatcher(ctx, k8sClient, namespace, []client.ObjectList{
		&corev1.EventList{},
		&batchv1.JobList{},
		&ytv1.YtsaurusList{},
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

func (w *NamespaceWatcher) Events() []testEvent {
	return w.events
}

func (w *NamespaceWatcher) loop() {
	for {
		select {
		case <-w.stopCh:
			return
		case ev := <-w.kubeWatcher.ResultChan():
			w.handleWatchEvent(ev)
		}
	}
}

func (w *NamespaceWatcher) handleWatchEvent(ev watch.Event) {
	w.maybeLogWatchEvent(ev)
	w.maybeStoreWatchEvent(ev)
}

func (w *NamespaceWatcher) maybeLogWatchEvent(ev watch.Event) {
	logEvent := func(event *corev1.Event) {
		log.Info("Event",
			"type", event.Type,
			"kind", event.InvolvedObject.Kind,
			"name", event.InvolvedObject.Name,
			"reason", event.Reason,
			"message", event.Message,
		)
	}

	switch ev.Type {
	case watch.Added, watch.Modified:
		if event, ok := ev.Object.(*corev1.Event); ok {
			logEvent(event)
		}
	}
}

func (w *NamespaceWatcher) maybeStoreWatchEvent(ev watch.Event) {
	switch ev.Type {
	case watch.Added, watch.Modified:
		if obj, ok := ev.Object.(*batchv1.Job); ok {
			event := testEvent{
				Kind: obj.Kind,
				Name: obj.Name,
			}
			w.events = append(w.events, event)
		}
	}
}
