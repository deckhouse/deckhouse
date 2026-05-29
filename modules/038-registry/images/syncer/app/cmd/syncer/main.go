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
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"syncer/pkg/config"
	"syncer/pkg/syncer"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func main() {
	// Args validation
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <config file>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	// Setup args
	cfgPath := os.Args[1]
	ctx := setupSignalHandler()

	// Start
	if err := runSync(ctx, newLogger(os.Stdout), cfgPath); err != nil {
		newLogger(os.Stderr).Error("sync failed", "error", err.Error())
		os.Exit(1)
	}
}

func runSync(ctx context.Context, logger *slog.Logger, cfgPath string) error {
	logger.Debug("Loading config", "path", cfgPath)
	cfg, err := config.FromFile(cfgPath)
	if err != nil {
		return fmt.Errorf("load config file %q: %w", cfgPath, err)
	}

	logger.Debug("Validating config", "path", cfgPath)
	err = cfg.Validate()
	if err != nil {
		return fmt.Errorf("validation config file %q: %w", cfgPath, err)
	}

	logger.Debug("Initializing syncer")
	syncer, err := syncer.New(
		logger,
		cfg,
	)
	if err != nil {
		return fmt.Errorf("create syncer: %w", err)
	}

	logger.Debug("Starting syncer")
	if err = syncer.Run(ctx); err != nil {
		return fmt.Errorf("sync error: %w", err)
	}
	return nil
}

func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
