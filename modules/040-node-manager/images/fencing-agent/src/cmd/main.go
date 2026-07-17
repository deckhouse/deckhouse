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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/fencingstate"
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/agent"
	"fencing-agent/internal/config"
)

const resolveIdentityTimeout = 30 * time.Second

func main() {
	cfg := &config.Config{}
	cfg.MustLoad()

	logger := newLogger(cfg.LogLevel)

	if err := run(cfg, logger); err != nil {
		logger.Error("fencing-agent failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *log.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	restCfg, err := kubeclient.NewRestConfig()
	if err != nil {
		return fmt.Errorf("build kubernetes rest config: %w", err)
	}

	var deps agent.Deps

	deps.K8sClient, err = kubeclient.New(restCfg)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}

	deps.FencingClient, err = fencingstate.NewClient(restCfg)
	if err != nil {
		return fmt.Errorf("create FencingNodeState client: %w", err)
	}

	resolveCtx, cancel := context.WithTimeout(ctx, resolveIdentityTimeout)
	defer cancel()

	identity, err := kubeclient.ResolveIdentity(resolveCtx, deps.K8sClient, cfg.NodeName)
	if err != nil {
		return fmt.Errorf("resolve node identity: %w", err)
	}

	cfg.NodeUID = identity.UID

	return agent.New(cfg, deps, identity, logger).Run(ctx)
}

func newLogger(level string) *log.Logger {
	return log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithLevel(log.LogLevelFromStr(level).Level()),
		log.WithHandlerType(log.JSONHandlerType),
	)
}
