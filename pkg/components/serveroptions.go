package components

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Option interface {
	apply(srv *serverImpl)
}

var (
	_ Option = &CustomReadinessProbeEndpoint{}
	_ Option = &ComponentContainerPorts{}
)

type CustomReadinessProbeEndpoint struct {
	port *intstr.IntOrString
	path *string
}

func (c CustomReadinessProbeEndpoint) apply(srv *serverImpl) {
	if c.port != nil {
		srv.readinessProbePort = *c.port
	}

	if c.path != nil {
		srv.readinessProbeHTTPPath = *c.path
	}
}

func WithCustomReadinessProbeEndpoint(port *intstr.IntOrString, path *string) Option {
	return CustomReadinessProbeEndpoint{
		port: port,
		path: path,
	}
}

type ComponentContainerPorts struct {
	ports []corev1.ContainerPort
}

func (c ComponentContainerPorts) apply(srv *serverImpl) {
	srv.componentContainerPorts = append(srv.componentContainerPorts, c.ports...)
}

func WithComponentContainerPorts(ports []corev1.ContainerPort) Option {
	return ComponentContainerPorts{
		ports: ports,
	}
}
