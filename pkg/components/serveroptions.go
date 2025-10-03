package components

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type options struct {
	containerPorts []corev1.ContainerPort

	readinessProbeEndpointPort intstr.IntOrString
	readinessProbeEndpointPath string

	sidecarImages map[string]string

	readinessByContainers []string
}

type Option func(opts *options)

func WithCustomReadinessProbeEndpointPort(port int32) Option {
	return func(opts *options) {
		opts.readinessProbeEndpointPort = intstr.FromInt32(port)
	}
}

func WithCustomReadinessProbeEndpointPath(path string) Option {
	return func(opts *options) {
		opts.readinessProbeEndpointPath = path
	}
}

func WithContainerPorts(ports ...corev1.ContainerPort) Option {
	return func(opts *options) {
		opts.containerPorts = append(opts.containerPorts, ports...)
	}
}

func WithSidecarImage(name, image string) Option {
	return func(opts *options) {
		if opts.sidecarImages == nil {
			opts.sidecarImages = make(map[string]string)
		}
		opts.sidecarImages[name] = image
	}
}

func WithReadinessByContainer(name string) Option {
	return func(opts *options) {
		opts.readinessByContainers = append(opts.readinessByContainers, name)
	}
}
