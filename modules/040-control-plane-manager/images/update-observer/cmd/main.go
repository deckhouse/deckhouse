package main

import (
	"context"
	"os/signal"
	"syscall"
	"update-observer/internal/constant"
	"update-observer/internal/manager"

	"k8s.io/klog/v2"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	manager, err := manager.NewManager(ctx, false) // TODO pprof flag
	if err != nil {
		klog.Fatalf("Failed to create a manager: %v", err)
	}

	if err = manager.Start(ctx); err != nil {
		klog.Fatalf("Failed to start the manager: %v", err)
	}

	<-ctx.Done()
	stop()
	klog.Infof("Bye from %s", constant.ControllerName)
}
