package components

import "context"

type microserviceComponentBase struct {
	componentBase
	microservice microservice
}

func (m *microserviceComponentBase) IsPodsRemovedConditionTrue() bool {
	return m.ytsaurus.IsUpdateStatusConditionTrue(m.labeller.GetPodsRemovedCondition())
}

func (m *microserviceComponentBase) NeedUpdate() bool {
	return m.microservice.needUpdate()
}

func (m *microserviceComponentBase) removePods(ctx context.Context) error {
	return removePods(ctx, m.microservice, &m.componentBase)
}

func (m *microserviceComponentBase) arePodsReady(ctx context.Context) bool {
	return m.microservice.arePodsReady(ctx)
}
