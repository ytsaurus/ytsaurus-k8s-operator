package components

import corev1 "k8s.io/api/core/v1"

type Option interface {
	apply(srv *serverImpl)
}

var (
	_ Option = &ReadinessProbeHTTPPath{}
	_ Option = &ComponentContainerPorts{}
)

type ReadinessProbeHTTPPath struct {
	path string
}

func (r ReadinessProbeHTTPPath) apply(srv *serverImpl) {
	srv.readinessProbeHTTPPath = r.path
}

func WithReadinessProbeHTTPPath(path string) Option {
	return ReadinessProbeHTTPPath{
		path: path,
	}
}

type ComponentContainerPorts struct {
	ports []corev1.ContainerPort
}

func (c ComponentContainerPorts) apply(srv *serverImpl) {
	for _, port := range c.ports {
		srv.componentContainerPorts = append(srv.componentContainerPorts, port)
	}
}

func WithComponentContainerPorts(ports []corev1.ContainerPort) Option {
	return ComponentContainerPorts{
		ports: ports,
	}
}
