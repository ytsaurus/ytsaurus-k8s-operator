package components

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
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
	GetName() string
}

type ComponentBase struct {
	labeller *labeller.Labeller
	apiProxy *apiproxy.APIProxy
	cfgen    *ytconfig.Generator
}

func (c *ComponentBase) GetName() string {
	return c.labeller.ComponentName
}
