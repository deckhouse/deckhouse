/*
Copyright 2024 Flant JSC

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
	"os"
	"os/signal"
	"syscall"

	"fencing-controller/internal/agent"
	"fencing-controller/internal/common"
	"fencing-controller/internal/watchdog/softdog"

	_ "github.com/jpfuentes2/go-env/autoload"
	"go.uber.org/zap"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logger := common.NewLogger()
	defer func() { _ = logger.Sync() }()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		s := <-sigChan
		close(sigChan)
		logger.Info("Got a signal", zap.String("signal", s.String()))
		cancel()
	}()

	var config agent.Config
	err := config.Load()
	if err != nil {
		logger.Fatal("Unable to read env vars", zap.Error(err))
	}

	logger.Debug("Current config", zap.Reflect("config", config))

	kubeClient, err := common.GetClientset(config.KubernetesAPITimeout)
	if err != nil {
		logger.Fatal("Unable to create a kubernetes clientSet", zap.Error(err))
	}

	wd := softdog.NewWatchdog(config.WatchdogDevice)
	fencingAgent := agent.NewFencingAgent(logger, config, kubeClient, wd)
	err = fencingAgent.Run(ctx)
	if err != nil {
		logger.Fatal("Unable run the fencing-agent", zap.Error(err))
	}
}
