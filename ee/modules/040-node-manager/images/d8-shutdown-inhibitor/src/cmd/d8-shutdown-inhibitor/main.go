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
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	dlog "github.com/deckhouse/deckhouse/pkg/log"

	"d8_shutdown_inhibitor/pkg/app"
	"d8_shutdown_inhibitor/pkg/kubernetes"
)

func run(cordonEnabled bool) error {
	nodeName, err := os.Hostname()
	if err != nil {
		dlog.Fatal("failed to get hostname", dlog.Err(err))
	}

	// Wait for kube-apiserver to be available before creating client
	var kubeClient *kubernetes.Klient
	dlog.Info("waiting for kube-apiserver to be available")

	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.5,
		Jitter:   0.1,
		Steps:    20, // ~5 minutes total
	}

	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		client, err := kubernetes.NewClientFromKubeconfig(kubernetes.KubeConfigPath)
		if err != nil {
			dlog.Warn("failed to create kubernetes client, retrying", dlog.Err(err))
			return false, nil
		}

		// Test connectivity by making a simple API call
		_, err = client.Clientset().Discovery().ServerVersion()
		if err != nil {
			dlog.Warn("kube-apiserver not ready, retrying", dlog.Err(err))
			return false, nil
		}

		kubeClient = client
		return true, nil
	})

	if err != nil {
		dlog.Fatal("failed to connect to kube-apiserver after retries", dlog.Err(err))
	}

	dlog.Info("successfully connected to kube-apiserver")
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
