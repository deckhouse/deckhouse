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
	"syscall"

	"github.com/spf13/cobra"

	dkplog "github.com/deckhouse/deckhouse/pkg/log"

	"github.com/deckhouse/deckhouse/go_lib/registry-bundle/pkg/log"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
)

func main() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	logLevel := slog.Level(
		dkplog.LogLevelFromStr(
			os.Getenv("LOG_LEVEL"),
		),
	)

	logHandler := dkplog.NewLogger(
		dkplog.WithHandlerType(
			dkplog.JSONHandlerType,
		),
		dkplog.WithLevel(
			logLevel,
		),
	).
		Named("registry").
		Handler()

	logger := log.NewSlog(logHandler)

	cmd := &cobra.Command{
		Use:           "registry",
		Short:         "OCI distribution registry from Deckhouse bundle",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	cmd.SetContext(setupSignalHandler(context.Background()))
	cmd.AddCommand(
		newServeCmd(logger),
		newValidateCmd(logger),
	)

	return cmd
}

func setupSignalHandler(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)

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
