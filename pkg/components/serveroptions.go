package components

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type options struct {
	containerPorts []corev1.ContainerPort

	readinessProbeEndpointPort intstr.IntOrString
	readinessProbeEndpointPath string
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
