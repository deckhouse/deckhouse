package main

import (
	"context"
	"fencing-agent/internal/app"
	fencing_config "fencing-agent/internal/config"
	"fencing-agent/internal/infrastructures/kubernetes"
	"fencing-agent/internal/infrastructures/logging"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := logging.NewLogger()
	defer func() { _ = logger.Sync() }()

	var config fencing_config.Config
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
		_ = os.Remove(config.GRPCAddress)
	}()
	if err := config.Load(); err != nil {
		logger.Fatal("Unable to read config", zap.Error(err))
	}
	kubeClient, err := kubernetes.GetClientset(config.KubernetesAPITimeout)
	if err != nil {
		logger.Fatal("Unable to create a kube-client", zap.Error(err))
	}
	application, err := app.NewApplication(logger, kubeClient, config)
	if err != nil {
		logger.Fatal("Unable to create an application", zap.Error(err))
	}
	if err = application.Run(ctx); err != nil {
		logger.Fatal("Unable to run the application", zap.Error(err))
	}
}
