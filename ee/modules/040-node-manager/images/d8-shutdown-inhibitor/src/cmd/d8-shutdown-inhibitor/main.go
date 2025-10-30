/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"os"
	"os/signal"
	"syscall"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/app"
	"d8_shutdown_inhibitor/pkg/kubernetes"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
	"github.com/spf13/cobra"
)

var (
    noCordon      bool
    cordonEnabled bool
)

var rootCmd = &cobra.Command{
    Use:   "d8-shutdown-inhibitor",
    Short: "Deckhouse shutdown inhibitor",
    RunE:  run,
}

func init() {
    rootCmd.Flags().BoolVar(&noCordon, "no-cordon", false, "Disable node cordoning")
}

func run(cmd *cobra.Command, args []string) error {
    cordonEnabled = !noCordon

    nodeName, err := os.Hostname()
    if err != nil {
        dlog.Fatal("failed to get hostname", dlog.Err(err))
    }

    // Start application.
    kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
    if err != nil {
        dlog.Fatal("failed to create kubernetes client", dlog.Err(err))
    }
    app := app.NewApp(app.AppConfig{
        PodLabel:              app.InhibitNodeShutdownLabel,
        InhibitDelayMax:       app.InhibitDelayMaxSec,
        PodsCheckingInterval:  app.PodsCheckingInterval,
        WallBroadcastInterval: app.WallBroadcastInterval,
        NodeName:              nodeName,
        CordonEnabled:         cordonEnabled,
    }, kubeClient)

    if err := app.Start(); err != nil {
        dlog.Fatal("application start failed", dlog.Err(err))
    }

    // Wait for signal to stop application.
    interruptCh := make(chan os.Signal, 1)
    signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)

    select {
    case sig := <-interruptCh:
        dlog.Info("received shutdown signal", slog.String("signal", sig.String()))
        app.Stop()
        <-app.Done()
    case <-app.Done():
        dlog.Info("application stopped by internal signal")
    }

    if err := app.Err(); err != nil {
        dlog.Fatal("application error", dlog.Err(err))
    }

    return nil
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
