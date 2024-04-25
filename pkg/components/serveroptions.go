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

func WithCustomReadinessProbeEndpoint(port *int32, path *string) Option {
	return func(opts *options) {
		if port != nil {
			opts.readinessProbeEndpointPort = intstr.FromInt32(*port)
		}

		if path != nil {
			opts.readinessProbeEndpointPath = *path
		}
	}
}

func WithContainerPorts(ports ...corev1.ContainerPort) Option {
	return func(opts *options) {
		opts.containerPorts = append(opts.containerPorts, ports...)
	}
}
