package components

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestImageHeaterValidatePods_NoPods(t *testing.T) {
	result := imageHeaterValidatePods(nil, map[string]struct{}{"img1": {}})
	if result.ok {
		t.Fatalf("expected not ok")
	}
	if result.reason != "Image heater daemonset has no pods yet" {
		t.Fatalf("unexpected reason: %s", result.reason)
	}
}

func TestImageHeaterValidatePods_TerminatingPod(t *testing.T) {
	pod := makeHeaterPod("ih-0", []string{"img1"})
	now := metav1.NewTime(time.Now())
	pod.DeletionTimestamp = &now
	result := imageHeaterValidatePods([]corev1.Pod{pod}, map[string]struct{}{"img1": {}})
	if result.ok {
		t.Fatalf("expected not ok")
	}
	if result.reason != "Image heater daemonset has terminating pod" {
		t.Fatalf("unexpected reason: %s", result.reason)
	}
	if result.podName != "ih-0" {
		t.Fatalf("unexpected pod name: %s", result.podName)
	}
}

func TestImageHeaterValidatePods_NoHeaterContainers(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ih-0"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "sidecar", Image: "img1"},
			},
		},
	}
	result := imageHeaterValidatePods([]corev1.Pod{pod}, map[string]struct{}{"img1": {}})
	if result.ok {
		t.Fatalf("expected not ok")
	}
	if result.reason != "Image heater daemonset pod has no heater containers" {
		t.Fatalf("unexpected reason: %s", result.reason)
	}
	if result.podName != "ih-0" {
		t.Fatalf("unexpected pod name: %s", result.podName)
	}
}

func TestImageHeaterValidatePods_MissingDesiredImage(t *testing.T) {
	pod := makeHeaterPod("ih-0", []string{"img2"})
	result := imageHeaterValidatePods([]corev1.Pod{pod}, map[string]struct{}{"img1": {}})
	if result.ok {
		t.Fatalf("expected not ok")
	}
	if result.reason != "Image heater daemonset pod missing desired image" {
		t.Fatalf("unexpected reason: %s", result.reason)
	}
	if result.image != "img1" {
		t.Fatalf("unexpected image: %s", result.image)
	}
}

func TestImageHeaterValidatePods_UnexpectedImage(t *testing.T) {
	pod := makeHeaterPod("ih-0", []string{"img1", "img2"})
	result := imageHeaterValidatePods([]corev1.Pod{pod}, map[string]struct{}{"img1": {}})
	if result.ok {
		t.Fatalf("expected not ok")
	}
	if result.reason != "Image heater daemonset pod has unexpected image" {
		t.Fatalf("unexpected reason: %s", result.reason)
	}
	if result.image != "img2" {
		t.Fatalf("unexpected image: %s", result.image)
	}
}

func TestImageHeaterValidatePods_Ok(t *testing.T) {
	pod := makeHeaterPod("ih-0", []string{"img1", "img2"})
	result := imageHeaterValidatePods([]corev1.Pod{pod}, map[string]struct{}{
		"img1": {},
		"img2": {},
	})
	if !result.ok {
		t.Fatalf("expected ok, got reason: %s", result.reason)
	}
}

func makeHeaterPod(name string, images []string) corev1.Pod {
	containers := make([]corev1.Container, 0, len(images))
	for _, image := range images {
		containers = append(containers, corev1.Container{
			Name:  imageHeaterContainerName(image),
			Image: image,
		})
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PodSpec{
			Containers: containers,
		},
	}
}
