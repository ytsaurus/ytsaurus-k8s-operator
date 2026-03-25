package components

import (
	"context"
)

// TODO: move to Updatable
type podsManager interface {
	removePods(ctx context.Context) error
	arePodsRemoved(ctx context.Context) bool
	arePodsReady(ctx context.Context) bool
	podsImageCorrespondsToSpec() bool
	arePodsUpdatedToNewRevision(ctx context.Context) bool
}
