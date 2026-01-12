package ports

import (
	"context"
	"fencing-controller/internal/core/domain"
)

type EventsBus interface {
	Publish(event domain.Event)
	Subscribe(ctx context.Context) <-chan domain.Event
}
