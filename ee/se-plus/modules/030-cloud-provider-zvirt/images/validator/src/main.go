/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
		dkplog.LevelInfo,
	)

	loghandler := dkplog.NewLogger(
		dkplog.WithHandlerType(dkplog.TextHandlerType),
		dkplog.WithLevel(logLevel),
	).Named("validator").Handler()

	logger := slog.New(loghandler)

	cmd := &cobra.Command{
		Use:           "validator",
		Short:         "zVirt cloud provider validator for dhctl",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	cmd.SetContext(setupSignalHandler(context.Background()))
	cmd.AddCommand(newServeCmd(logger))

	return cmd
}

// setupSignalHandler cancels the context on the first signal and gives up on the second: a
// validator that will not stop would hold up whatever dhctl is doing.
func setupSignalHandler(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)

	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)

	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1)
	}()

	return ctx
}
