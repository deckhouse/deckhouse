/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package static_pod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	authTemplatePath         = "/templates/auth_config/config.yaml.tpl"
	distributionTemplatePath = "/templates/distribution_config/config.yaml.tpl"
	staticPodTemplatePath    = "/templates/static_pods/system-registry.yaml.tpl"
	authConfigPath           = "/etc/kubernetes/system-registry/auth_config/config.yaml"
	distributionConfigPath   = "/etc/kubernetes/system-registry/distribution_config/config.yaml"
	staticPodConfigPath      = "/etc/kubernetes/manifests/system-registry.yaml"
	pkiConfigDirectoryPath   = "/etc/kubernetes/system-registry/pki"

	distributionConfiguration = "distributionConfiguration"
	authConfiguration         = "authConfiguration"
	pkiFiles                  = "pkiFiles"
	staticPodConfiguration    = "staticPodConfiguration"
)

type apiServer struct {
	requestMutex sync.Mutex
}

func Run(ctx context.Context) error {
	log := ctrl.Log.WithValues("component", "static pod manager")

	log.Info("Starting static pod manager")

	var api apiServer

	http.HandleFunc("/staticpod/create", api.CreateStaticPodHandler)
	http.HandleFunc("/staticpod/delete", api.DeleteStaticPodHandler)

	httpServer := &http.Server{Addr: "127.0.0.1:4576"}

	context.AfterFunc(ctx, func() {
		log.Info("Shutting down API server")

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Error(err, "Error shutting down API server")
		}
	})

	log.Info("Starting API server on %v", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start API server: %w", err)
	}

	return nil
}

func (s *apiServer) CreateStaticPodHandler(w http.ResponseWriter, r *http.Request) {

	ctrl.Log.Info("Received request to create/update static pod and configuration", "Client address:", r.RemoteAddr, "component", "static pod manager")

	if r.Method != http.MethodPost {
		ctrl.Log.Info("method not allowed", "component", "static pod manager", "endpoint", r.RequestURI, "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	var data EmbeddedRegistryConfig
	// Decode request body to struct EmbeddedRegistryConfig and return error if decoding fails
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		ctrl.Log.Info("Error decoding request body", "error", err, "component", "static pod manager")
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Fill the host IP address from the HOST_IP environment variable
	hostIpAddress, err := data.fillHostIpAddress()
	if err != nil {
		ctrl.Log.Error(err, "Error getting IP address")
		http.Error(w, "Internal server error: fillHostIpAddress", http.StatusInternalServerError)
		return
	}
	data.IpAddress = hostIpAddress

	// Validate the request data
	if err := data.validate(); err != nil {
		ctrl.Log.Info("Request validation error", "error", err, "component", "static pod manager")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	changes := make(map[string]bool)

	// Save the PKI files
	changes[pkiFiles], err = data.Pki.savePkiFiles(pkiConfigDirectoryPath, &data.ConfigHashes)
	if err != nil {
		ctrl.Log.Error(err, "Error saving PKI files", "component", "static pod manager")
		http.Error(w, "Error saving PKI files", http.StatusInternalServerError)
		return
	}

	// Process the templates with the given data and create the static pod and configuration files

	changes[authConfiguration], err = data.processTemplate(authTemplatePath, authConfigPath, &data.ConfigHashes.AuthTemplateHash)
	if err != nil {
		ctrl.Log.Error(err, "Error processing auth template", "component", "static pod manager")
		http.Error(w, "Error processing auth template", http.StatusInternalServerError)
		return
	}

	changes[distributionConfiguration], err = data.processTemplate(distributionTemplatePath, distributionConfigPath, &data.ConfigHashes.DistributionTemplateHash)
	if err != nil {
		ctrl.Log.Error(err, "Error processing distribution template", "component", "static pod manager")
		http.Error(w, "Error processing distribution template", http.StatusInternalServerError)
		return
	}

	changes[staticPodConfiguration], err = data.processTemplate(staticPodTemplatePath, staticPodConfigPath, nil)
	if err != nil {
		ctrl.Log.Error(err, "Error processing static pod template", "component", "static pod manager")
		http.Error(w, "Error processing static pod template", http.StatusInternalServerError)
		return
	}

	if hasChanges(changes) {
		ctrl.Log.Info("Static pod and configuration created/updated successfully", "component", "static pod manager")
	} else {
		ctrl.Log.Info("No changes in static pod and configuration", "component", "static pod manager")
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"changes": changes,
	})
	if err != nil {
		ctrl.Log.Error(err, "Error encoding response", "component", "static pod manager")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func (s *apiServer) DeleteStaticPodHandler(w http.ResponseWriter, r *http.Request) {

	ctrl.Log.Info("Received request to delete static pod and configuration", "Client address:", r.RemoteAddr, "component", "static pod manager")

	if r.Method != http.MethodDelete {
		ctrl.Log.Info("method not allowed", "component", "static pod manager", "endpoint", r.RequestURI, "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	var err error
	changes := make(map[string]bool)

	// Delete the static pod file
	changes[staticPodConfiguration], err = deleteFile(staticPodConfigPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting static pod file", "component", "static pod manager")
		http.Error(w, "Error deleting static pod file", http.StatusInternalServerError)
		return
	}

	// Delete the auth config file
	changes[authConfiguration], err = deleteFile(authConfigPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting auth config file", "component", "static pod manager")
		http.Error(w, "Error deleting auth config file", http.StatusInternalServerError)
		return
	}

	// Delete the distribution config file
	changes[distributionConfiguration], err = deleteFile(distributionConfigPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting distribution config file", "component", "static pod manager")
		http.Error(w, "Error deleting distribution config file", http.StatusInternalServerError)
		return
	}

	changes[pkiFiles], err = deleteDirectory(pkiConfigDirectoryPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting registry pki directory", "component", "static pod manager")
		http.Error(w, "Error deleting registry pki directory", http.StatusInternalServerError)
		return
	}

	if hasChanges(changes) {
		ctrl.Log.Info("Static pod and configuration deleted successfully", "component", "static pod manager")
	} else {
		ctrl.Log.Info("No static pod and configuration to delete", "component", "static pod manager")
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"changes": changes,
	})
	if err != nil {
		ctrl.Log.Error(err, "Error encoding response", "component", "static pod manager")
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
