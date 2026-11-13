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

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/agent"
	"fencing-agent/internal/config"
)

func main() {
	logger := log.NewLogger(
		log.WithOutput(os.Stdout),
		log.WithHandlerType(log.JSONHandlerType),
	)

	if err := run(logger); err != nil {
		logger.Error("fencing-agent failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	env, err := config.LoadEnv()
	if err != nil {
		return err
	}

	logger.SetLevel(log.LogLevelFromStr(env.LogLevel))
	// client-go writes through klog and the controller-runtime client writes through
	// its own logger. Both write JSON through the agent logger.
	bridge := logr.FromSlogHandler(logger.Handler())
	ctrllog.SetLogger(bridge)
	klog.SetLogger(bridge)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	restCfg, err := kubeclient.NewRestConfig()
	if err != nil {
		return fmt.Errorf("build kubernetes rest config: %w", err)
	}

	deps := agent.Deps{RestConfig: restCfg}

	deps.K8sClient, err = kubeclient.New(restCfg)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}

	a, err := agent.New(env, deps, logger)
	if err != nil {
		return err
	}

	return a.Run(ctx)
}
