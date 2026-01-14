package ports

import (
	"context"
	"fencing-agent/internal/core/domain"
)

type StatusQuery interface {
	GetAllNodes(ctx context.Context) ([]domain.Node, error)
}
