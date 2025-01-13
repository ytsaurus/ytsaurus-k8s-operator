package controllers_test

import (
	"context"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceWatcher struct {
	kubeWatcher watch.Interface
	stopCh      chan struct{}
	events      []WatcherEvent
}

type WatcherEvent struct {
	Kind string
	Name string
}

func NewNamespaceWatcher(ctx context.Context, namespace string) *NamespaceWatcher {
	watcher, err := k8sClient.Watch(ctx, &corev1.EventList{}, &client.ListOptions{
		Namespace: namespace,
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

func (w *NamespaceWatcher) Events() []WatcherEvent {
	return w.events
}

func (w *NamespaceWatcher) loop() {
	for {
		select {
		case <-w.stopCh:
			return
		case ev := <-w.kubeWatcher.ResultChan():
			w.handleEvent(ev)
		}
	}
}

func (w *NamespaceWatcher) handleEvent(ev watch.Event) {
	w.maybeLogEvent(ev)
	w.maybeStoreEvent(ev)
}

func (w *NamespaceWatcher) maybeLogEvent(ev watch.Event) {
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

func (w *NamespaceWatcher) maybeStoreEvent(ev watch.Event) {
	switch ev.Type {
	case watch.Added, watch.Modified:
		if event, ok := ev.Object.(*corev1.Event); ok {
			w.events = append(w.events, WatcherEvent{
				Kind: event.InvolvedObject.Kind,
				Name: event.InvolvedObject.Name,
			})
		}
	}
}
