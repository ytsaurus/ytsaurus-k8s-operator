package components

import (
	"context"
)

type serverComponentBase struct {
	componentBase
	server server
}

func (s *serverComponentBase) IsPodsRemovedConditionTrue() bool {
	return s.ytsaurus.IsUpdateStatusConditionTrue(s.labeller.GetPodsRemovedCondition())
}

func (s *serverComponentBase) NeedUpdate() bool {
	return s.server.needUpdate()
}

func (s *serverComponentBase) removePods(ctx context.Context) error {
	return removePods(ctx, s.server, &s.componentBase)
}

func (s *serverComponentBase) arePodsReady(ctx context.Context) bool {
	return s.server.arePodsReady(ctx)
}
