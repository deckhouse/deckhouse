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
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/rest"

	"node-services-manager/internal/staticpod"
)

var (
	shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
	log             = slog.With("component", "main")
)

func main() {
	var settings staticpod.AppSettings

	settings.HostIP = getEnvOrExit("HOST_IP")
	log = log.With("ip", settings.HostIP)

	settings.NodeName = getEnvOrExit("NODE_NAME")
	log = log.With("node", settings.NodeName)

	settings.PodName = getEnvOrExit("POD_NAME")
	log = log.With("pod.name", settings.PodName)

	settings.PodNamespace = getEnvOrExit("POD_NAMESPACE")
	log = log.With("pod.namespace", settings.PodNamespace)

	settings.ImageAuth = getEnvOrExit("IMAGE_AUTH")
	settings.ImageDistribution = getEnvOrExit("IMAGE_DISTRIBUTION")
	settings.ImageMirrorer = getEnvOrExit("IMAGE_MIRRORER")

	// Load Kubernetes configuration
	cfg, err := rest.InClusterConfig()
	if err != nil {
		log.Error("Unable to get kubeconfig", "error", err)
		os.Exit(1)
	}

	log.Info("Starting Node Services manager")

	log.Info("Setup signal handler")
	ctx := setupSignalHandler()

	log.Info("Starting application")

	if err = staticpod.Run(ctx, cfg, settings); err != nil {
		log.Error("Application error", "error", err)
	}

	log.Info("Bye!")

	if err != nil {
		os.Exit(1)
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

func getEnvOrExit(name string) string {
	val := os.Getenv(name)
	if val == "" {
		log.Error("Required environment variable is not set", "variable", name)
		os.Exit(1)
	}

	return val
}
