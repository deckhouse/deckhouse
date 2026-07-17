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

// Package agent holds the runtime context of fencing-agent assembled by the
// composition root in cmd. Fencing flow components (memberlist client,
// watchdog manager, FencingNodeState writer, rejoin loop, local Kubernetes
// Style API server) are attached here by follow-up tasks.
package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/config"
	"fencing-agent/internal/controllers/health"
	"fencing-agent/internal/domain"
)

type Agent struct {
	cfg      *config.Config
	deps     Deps
	identity domain.NodeIdentity
	logger   *log.Logger
	health   *health.Server
}

func New(cfg *config.Config, deps Deps, identity domain.NodeIdentity, logger *log.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		deps:     deps,
		identity: identity,
		logger:   logger,
		health:   health.NewServer(cfg.HealthProbeBindAddress, logger),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	if a.deps.K8sClient == nil || a.deps.FencingClient == nil {
		return errors.New("agent dependencies are not wired: K8sClient and FencingClient are required")
	}

	a.logger.Info("fencing-agent context initialized, fencing flow is not started",
		"node", a.identity.Name,
		"nodeUID", a.identity.UID,
		"nodeGroup", a.cfg.NodeGroup,
		"profile", a.cfg.ProfileRefName,
		"watchdogDevice", a.cfg.WatchdogDevice,
		"apiSocketPath", a.cfg.APISocketPath,
	)

	if err := a.health.Run(ctx); err != nil {
		return fmt.Errorf("health server: %w", err)
	}

	a.logger.Info("fencing-agent stopped")

	return nil
}
