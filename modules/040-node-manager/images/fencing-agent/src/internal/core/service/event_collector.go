package service

import (
	"fencing-controller/internal/core/ports"

	"go.uber.org/zap"
)

type EventCollector struct {
	logger *zap.Logger
}

func NewEventCollector(eventBus ports.EventsBus, logger *zap.Logger) *EventCollector {
	return &EventCollector{logger: logger}
}
