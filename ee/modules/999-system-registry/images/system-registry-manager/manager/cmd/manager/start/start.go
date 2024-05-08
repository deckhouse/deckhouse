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
	"system-registry-manager/internal/kubeapi"
	"system-registry-manager/internal/steps"

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
		if err := StartManager(); err != nil {
			log.Errorf("Manager error: %v", err)
			// TODO
			time.Sleep(10 * time.Second)
		}
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

func StartManager() error {
	cfg := config.GetConfig()

	if err := steps.PrepareWorkspace(); err != nil {
		return err
	}
	if err := steps.GenerateCerts(); err != nil {
		return err
	}
	if err := steps.CheckDestFiles(); err != nil {
		return err
	}
	if !((cfg.ShouldUpdateBy.NeedChangeFileByExist ||
		cfg.ShouldUpdateBy.NeedChangeFileByCheckSum) ||
		(cfg.ShouldUpdateBy.NeedChangeSeaweedfsCerts ||
			cfg.ShouldUpdateBy.NeedChangeDockerAuthTokenCerts)) {
		return nil
	}

	if err := kubeapi.SetMyStatusAndWaitApprove("update", 0); err != nil {
		return err
	}
	if err := steps.UpdateManifests(); err != nil {
		return err
	}
	if err := kubeapi.SetMyStatusDone(); err != nil {
		return err
	}
	return nil
}
