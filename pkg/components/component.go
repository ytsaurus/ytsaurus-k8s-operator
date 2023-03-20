package components

import (
	"context"
)

type SyncStatus string

const (
	SyncStatusBlocked SyncStatus = "Blocked"
	SyncStatusPending SyncStatus = "Pending"
	SyncStatusReady   SyncStatus = "Ready"
)

type Component interface {
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) SyncStatus
}
