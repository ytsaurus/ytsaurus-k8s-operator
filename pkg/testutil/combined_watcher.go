package testutil

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CombinedKubeWatcher is a watcher that combines multiple watchers into a single channel preserving watcher interface.
type CombinedKubeWatcher struct {
	watchers      []watch.Interface
	stopCh        chan struct{}
	muxResultChan chan watch.Event
}

func NewCombinedKubeWatcher(ctx context.Context, kubecli client.WithWatch, namespace string, lists []client.ObjectList) (*CombinedKubeWatcher, error) {
	muxResultChan := make(chan watch.Event)
	stopCh := make(chan struct{})

	var watchers []watch.Interface
	for _, objList := range lists {
		watcher, err := kubecli.Watch(ctx, objList, &client.ListOptions{
			Namespace: namespace,
		})
		if err != nil {
			return nil, err
		}
		go func(ch <-chan watch.Event) {
			for {
				select {
				case msg, ok := <-ch:
					if !ok {
						return
					}
					muxResultChan <- msg
				case <-stopCh:
					return
				}
			}
		}(watcher.ResultChan())
		watchers = append(watchers, watcher)
	}
	return &CombinedKubeWatcher{
		watchers:      watchers,
		stopCh:        stopCh,
		muxResultChan: muxResultChan,
	}, nil
}

func (w *CombinedKubeWatcher) Stop() {
	close(w.stopCh)
	for _, watcher := range w.watchers {
		watcher.Stop()
	}
}

func (w *CombinedKubeWatcher) ResultChan() <-chan watch.Event {
	return w.muxResultChan
}
