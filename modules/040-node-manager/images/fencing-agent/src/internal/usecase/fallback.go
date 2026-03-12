/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
