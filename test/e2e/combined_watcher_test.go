package controllers_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type combinedKubeWatcher struct {
	watchers      []watch.Interface
	stopCh        chan struct{}
	muxResultChan chan watch.Event
}

func newCombinedKubeWatcher(ctx context.Context, kubecli client.WithWatch, namespace string, lists []client.ObjectList) (*combinedKubeWatcher, error) {
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
	return &combinedKubeWatcher{
		watchers:      watchers,
		stopCh:        stopCh,
		muxResultChan: muxResultChan,
	}, nil
}

func (w *combinedKubeWatcher) Stop() {
	close(w.stopCh)
	for _, watcher := range w.watchers {
		watcher.Stop()
	}
}

func (w *combinedKubeWatcher) ResultChan() <-chan watch.Event {
	return w.muxResultChan
}

func TestCombinedWatcher(t *testing.T) {
	testCtx := context.Background()

	cfg, err := config.GetConfig()
	require.NoError(t, err)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	kubecli, err := client.NewWithWatch(cfg, client.Options{Scheme: scheme})
	require.NoError(t, err)

	// Create a temporary namespace
	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-namespace-",
		},
	}
	err = kubecli.Create(testCtx, namespaceObj)
	require.NoError(t, err)
	namespace := namespaceObj.Name
	defer func() {
		err := kubecli.Delete(testCtx, namespaceObj)
		require.NoError(t, err)
	}()

	watcher, err := newCombinedKubeWatcher(testCtx, kubecli, namespace, []client.ObjectList{
		&corev1.ConfigMapList{},
		&corev1.SecretList{},
	})
	require.NoError(t, err)

	var events []watch.Event
	go func() {
		for {
			select {
			case msg, ok := <-watcher.ResultChan():
				if !ok {
					return
				}
				switch obj := msg.Object.(type) {
				case *corev1.ConfigMap:
					if obj.Name == "kube-root-ca.crt" {
						continue
					}
				}
				events = append(events, msg)
			}
		}
	}()

	err = kubecli.Create(testCtx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: namespace,
		},
	}, &client.CreateOptions{})
	require.NoError(t, err)
	err = kubecli.Create(testCtx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: namespace,
		},
	}, &client.CreateOptions{})
	require.NoError(t, err)

	expectedEvents := []watch.Event{
		{Type: watch.Added, Object: &corev1.ConfigMap{}},
		{Type: watch.Added, Object: &corev1.Secret{}},
	}
	require.Eventually(t, func() bool {
		if len(events) > len(expectedEvents) {
			t.Errorf("Got more events than expected already: %s", events)
		}
		for i, realEvent := range events {
			expectedEvent := expectedEvents[i]
			realObjectType := reflect.TypeOf(realEvent.Object)
			expectedObjectType := reflect.TypeOf(expectedEvent.Object)
			if realEvent.Type != expectedEvent.Type || realObjectType != expectedObjectType {
				t.Errorf("Event %d did not match expected event: %v != %v", i, realEvent, expectedEvent)
			}
		}
		return len(events) == len(expectedEvents)
	}, 3*time.Second, 100*time.Millisecond, "Events never matched expected list, got: %+v", events)
}
