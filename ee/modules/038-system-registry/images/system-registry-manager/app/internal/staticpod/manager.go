/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

const (
	listenAddr = "127.0.0.1:4576"

	authConfigPath         = "/etc/kubernetes/system-registry/auth_config/config.yaml"
	distributionConfigPath = "/etc/kubernetes/system-registry/distribution_config/config.yaml"
	pkiConfigDirectoryPath = "/etc/kubernetes/system-registry/pki"

	registryStaticPodConfigPath = "/etc/kubernetes/manifests/system-registry.yaml"

	distributionConfiguration = "distributionConfiguration"
	authConfiguration         = "authConfiguration"
	pkiFiles                  = "pkiFiles"
	staticPodConfiguration    = "staticPodConfiguration"
)

type apiServer struct {
	requestMutex sync.Mutex
	log          *dlog.Logger
}

func Run(ctx context.Context) error {
	log := dlog.Default().With("component", "static pod manager")

	log.Info("Starting")
	defer log.Info("Stopped")

	api := &apiServer{
		log: log,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/staticpod", api.handleStaticPod)

	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	context.AfterFunc(ctx, func() {
		log.Info("Shutting down API server")

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Error("Error shutting down API server", "error", err)
		}
	})

	log.Info("Starting API server", "addr", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("API server error", err)
		return fmt.Errorf("failed to start API server: %w", err)
	}

	return nil
}

func (s *apiServer) handleStaticPod(w http.ResponseWriter, r *http.Request) {
	log := s.log.With("endpoint", r.RequestURI, "method", r.Method)

	switch r.Method {
	case http.MethodPost:
		s.handleStaticPodPost(w, r)
	case http.MethodDelete:
		s.handleStaticPodDelete(w, r)
	default:
		log.Warn("Method not allowed")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *apiServer) handleStaticPodPost(w http.ResponseWriter, r *http.Request) {
	log := s.log.With("handler", "create")

	log.Info("Received request to create/update static pod and configuration", "Client address:", r.RemoteAddr)

	if r.Method != http.MethodPost {
		log.Warn("Method not allowed", "endpoint", r.RequestURI, "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	var data EmbeddedRegistryConfig
	// Decode request body to struct EmbeddedRegistryConfig and return error if decoding fails
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Warn("Error decoding request body", "error", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Fill the host IP address from the HOST_IP environment variable
	hostIpAddress, err := data.fillHostIpAddress()
	if err != nil {
		log.Error("Error getting IP address", "error", err)
		http.Error(w, "Internal server error: fillHostIpAddress", http.StatusInternalServerError)
		return
	}
	data.IpAddress = hostIpAddress

	// Validate the request data
	if err := data.validate(); err != nil {
		log.Warn("Request validation error", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	changes := make(map[string]bool)

	// Save the PKI files
	changes[pkiFiles], err = data.Pki.savePkiFiles(pkiConfigDirectoryPath, &data.ConfigHashes)
	if err != nil {
		log.Error("Error saving PKI files", "error", err)
		http.Error(w, "Error saving PKI files", http.StatusInternalServerError)
		return
	}

	// Process the templates with the given data and create the static pod and configuration files

	changes[authConfiguration], err = data.processTemplate(
		authConfigTemplateName,
		authConfigPath,
		&data.ConfigHashes.AuthTemplateHash,
	)

	if err != nil {
		log.Error("Error processing auth template", "error", err)
		http.Error(w, "Error processing auth template", http.StatusInternalServerError)
		return
	}

	changes[distributionConfiguration], err = data.processTemplate(
		distributionConfigTemplateName,
		distributionConfigPath,
		&data.ConfigHashes.DistributionTemplateHash,
	)

	if err != nil {
		log.Error("Error processing distribution template", "error", err)
		http.Error(w, "Error processing distribution template", http.StatusInternalServerError)
		return
	}

	changes[staticPodConfiguration], err = data.processTemplate(
		registryStaticPodTemplateName,
		registryStaticPodConfigPath,
		nil,
	)

	if err != nil {
		log.Error("Error processing static pod template", "error", err)
		http.Error(w, "Error processing static pod template", http.StatusInternalServerError)
		return
	}

	if hasChanges(changes) {
		log.Info("Static pod and configuration created/updated successfully")
	} else {
		log.Info("No changes in static pod and configuration")
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).
		Encode(map[string]interface{}{
			"changes": changes,
		})

	if err != nil {
		log.Error("Error encoding response", "error", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func (s *apiServer) handleStaticPodDelete(w http.ResponseWriter, r *http.Request) {
	log := s.log.With("handler", "delete")

	log.Info("Received request to delete static pod and configuration", "Client address:", r.RemoteAddr)

	if r.Method != http.MethodDelete {
		log.Warn("method not allowed", "endpoint", r.RequestURI, "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	var err error
	changes := make(map[string]bool)

	// Delete the static pod file
	changes[staticPodConfiguration], err = deleteFile(registryStaticPodConfigPath)
	if err != nil {
		log.Error("Error deleting static pod file", "error", err)
		http.Error(w, "Error deleting static pod file", http.StatusInternalServerError)
		return
	}

	// Delete the auth config file
	changes[authConfiguration], err = deleteFile(authConfigPath)
	if err != nil {
		log.Error("Error deleting auth config file", "error", err)
		http.Error(w, "Error deleting auth config file", http.StatusInternalServerError)
		return
	}

	// Delete the distribution config file
	changes[distributionConfiguration], err = deleteFile(distributionConfigPath)
	if err != nil {
		log.Error("Error deleting distribution config file", "error", err)
		http.Error(w, "Error deleting distribution config file", http.StatusInternalServerError)
		return
	}

	changes[pkiFiles], err = deleteDirectory(pkiConfigDirectoryPath)
	if err != nil {
		log.Error("Error deleting registry pki directory", "error", err)
		http.Error(w, "Error deleting registry pki directory", http.StatusInternalServerError)
		return
	}

	if hasChanges(changes) {
		log.Info("Static pod and configuration deleted successfully")
	} else {
		log.Info("No static pod and configuration to delete")
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).
		Encode(map[string]interface{}{
			"changes": changes,
		})

	if err != nil {
		log.Error("Error encoding response", "error", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func hasChanges(changes map[string]bool) bool {
	for _, changed := range changes {
		if changed {
			return true
		}
	}
	return false
}
