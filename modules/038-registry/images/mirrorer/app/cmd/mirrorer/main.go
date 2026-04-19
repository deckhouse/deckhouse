/*
Copyright 2025 Flant JSC

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

	"mirrorer/internal/config"
	"mirrorer/internal/mirrorer"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func main() {
	log := slog.With("component", "main")

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %v <config file>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	configFile := os.Args[1]
	log.Debug("Loading config", "config_file", configFile)

	cfg, err := config.FromFile(configFile)
	if err != nil {
		log.Error("Cannot load config file", "config_file", configFile, "error", err)
		os.Exit(1)
	}

	err = cfg.Validate()
	if err != nil {
		log.Error("Config validation error", "config_file", configFile, "error", err)
		os.Exit(1)
	}

	worker, err := mirrorer.New(slog.Default(), cfg)
	if err != nil {
		log.Error("Cannot create mirrorer", "error", err)
		os.Exit(2)
	}

	log.Info("Setup signal handler")
	ctx := setupSignalHandler()

	log.Info("Starting mirrorer")
	if err = worker.Run(ctx); err != nil {
		log.Error("Mirrorer error", "error", err)
	}

	log.Info("Bye!")

	if err != nil {
		os.Exit(3)
	}
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
