package usecase

import (
	"context"
	"fencing-agent/internal/lib/logger/sl"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type KubeClient interface {
	GetNodesIP(ctx context.Context) ([]string, error)
}

type Fallback struct {
	kubeClient KubeClient
	logger     *log.Logger
}

func NewFallback(logger *log.Logger, kubeClient KubeClient) *Fallback {
	return &Fallback{kubeClient: kubeClient, logger: logger}
}

func (fb *Fallback) ShouldFeed(ctx context.Context) bool {
	_, err := fb.kubeClient.GetNodesIP(ctx)
	if err != nil {
		fb.logger.Debug("kubernetes API is not available", sl.Err(err))

		return false
	}

	return true
}
