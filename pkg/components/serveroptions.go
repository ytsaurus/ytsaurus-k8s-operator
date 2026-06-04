package components

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type options struct {
	containerPorts []corev1.ContainerPort

	readinessProbeEndpointPort intstr.IntOrString
	readinessProbeEndpointPath string

	sidecarImages map[string]string

	// timbertruck is a per-component override of the cluster-wide timbertruck settings.
	// Only masters set it (for backward compatibility); other components rely on the
	// cluster-wide spec.timbertruck plus per-log enableDelivery flags.
	timbertruck *ytv1.TimbertruckSpec
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

// WithTimbertruck sets a per-component override of the cluster-wide timbertruck settings.
func WithTimbertruck(timbertruck *ytv1.TimbertruckSpec) Option {
	return func(opts *options) {
		opts.timbertruck = timbertruck
	}
}
