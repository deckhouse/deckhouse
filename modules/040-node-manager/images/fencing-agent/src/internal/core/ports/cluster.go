package ports

import (
	"context"
	"fencing-agent/internal/core/domain"
)

type ClusterProvider interface {
	GetNodes(ctx context.Context) ([]domain.Node, error)
	IsAvailable(ctx context.Context) bool
	IsMaintenanceMode(ctx context.Context) (bool, error)
	SetNodeLabel(ctx context.Context, key, value string) error
	RemoveNodeLabel(ctx context.Context, key string) error
}
