/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"d8_shutdown_inhibitor/pkg/app"
	"d8_shutdown_inhibitor/pkg/kubernetes"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

func run(cordonEnabled bool) error {
	nodeName, err := os.Hostname()
	if err != nil {
		dlog.Fatal("failed to get hostname", dlog.Err(err))
	}

	// Start application.
	kubeClient, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
	if err != nil {
		dlog.Fatal("failed to create kubernetes client", dlog.Err(err))
	}
	a := app.NewApp(app.AppConfig{
		PodLabel:              app.InhibitNodeShutdownLabel,
		InhibitDelayMax:       app.InhibitDelayMaxSec,
		PodsCheckingInterval:  app.PodsCheckingInterval,
		WallBroadcastInterval: app.WallBroadcastInterval,
		NodeName:              nodeName,
		CordonEnabled:         cordonEnabled,
	}, kubeClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interruptCh)

	if err := a.Start(ctx, cancel); err != nil {
		dlog.Fatal("application start failed", dlog.Err(err))
	}

	select {
	case sig := <-interruptCh:
		dlog.Info("received shutdown signal", slog.String("signal", sig.String()))
		cancel()
		a.Stop()
		<-a.Done()
	case <-a.Done():
		dlog.Info("application stopped by internal signal")
	}

	if err := a.Err(); err != nil {
		dlog.Fatal("application error", dlog.Err(err))
	}

	return nil
}

func main() {
	noCordon := flag.Bool("no-cordon", false, "Disable node cordoning")
	flag.Parse()

	cordonEnabled := !*noCordon

	if err := run(cordonEnabled); err != nil {
		os.Exit(1)
	}
}
