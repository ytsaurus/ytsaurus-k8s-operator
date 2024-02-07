package components

import (
	"context"
)

func (s *serverImpl) Status(ctx context.Context) ComponentStatus {

	return ComponentStatus{
		SyncStatus: SyncStatusReady,
	}
}
