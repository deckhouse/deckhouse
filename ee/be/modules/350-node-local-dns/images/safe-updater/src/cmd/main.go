/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	manager "safe-updater/internal/manager"
)

func main() {
	ppprofFlagPtr := flag.Bool("pprof", false, "enable pprof")
	klog.InitFlags(nil)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())

	manager, err := manager.NewManager(ctx, *ppprofFlagPtr)
	if err != nil {
		klog.Fatalf("Failed to create a manager: %v", err)
	}

	if err = manager.Start(ctx); err != nil {
		klog.Fatalf("Failed to start the manager: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for range sigs {
		cancel()
		klog.Info("Bye from safe-updater")
		break
	}
}
