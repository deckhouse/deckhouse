/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package start

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"system-registry-manager/internal/config"
	"system-registry-manager/internal/steps"
	kube_actions "system-registry-manager/pkg/kubernetes/actions"

	log "github.com/sirupsen/logrus"
)

var (
	server                     *http.Server
	controlPlaneManagerIsReady = false
)

func Start() {
	// Initialize logger
	initLogger()

	log.Info("Start service")
	log.Infof("Config file: %s", config.GetConfigFilePath())

	// Initialize configuration
	if err := config.InitConfig(); err != nil {
		log.Fatalf("Error initializing config: %v", err)
	}

	// Create HTTP server
	server = &http.Server{
		Addr: "127.0.0.1:8097",
	}

	// Define HTTP routes
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/readyz", readyzHandler)

	// Graceful shutdown
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Info("Shutting down server...")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Errorf("Error shutting down server: %v", err)
		}
	}()

	// Start HTTP server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("Error starting server: %v", err)
		}
	}()

	// Start manager
	for {
		if err := startManager(); err != nil {
			log.Errorf("Manager error: %v", err)
		}
		// TODO
		time.Sleep(10 * time.Second)
		log.Info("Wait for 10 seconds...")
	}
}

func initLogger() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.JSONFormatter{})
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyzHandler(w http.ResponseWriter, _ *http.Request) {
	if controlPlaneManagerIsReady {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

func startManager() error {
	manifestsSpec := config.NewManifestsSpec()

	if err := steps.PrepareWorkspace(manifestsSpec); err != nil {
		return err
	}
	if err := steps.GenerateCerts(manifestsSpec); err != nil {
		return err
	}
	if err := steps.CheckDestFiles(manifestsSpec); err != nil {
		return err
	}
	if !manifestsSpec.NeedChange() {
		log.Info("No changes")
		return nil
	}

	if err := kube_actions.SetMyStatusAndWaitApprove("update", 0); err != nil {
		return err
	}
	if err := steps.UpdateManifests(manifestsSpec); err != nil {
		return err
	}
	if err := kube_actions.SetMyStatusDone(); err != nil {
		return err
	}
	return nil
}
