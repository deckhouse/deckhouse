package static_pod

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sync"
)

const authTemplatePath = "/templates/auth_config/config.yaml.tpl"
const distributionTemplatePath = "/templates/distribution_config/config.yaml.tpl"
const staticPodTemplatePath = "/templates/static_pods/system-registry.yaml.tpl"
const authConfigPath = "/etc/kubernetes/system-registry/auth_config/config.yaml"
const distributionConfigPath = "/etc/kubernetes/system-registry/distribution_config/config.yaml"
const staticPodConfigPath = "/etc/kubernetes/manifests/system-registry.yaml"

type Server struct {
	KubeClient   *kubernetes.Clientset
	requestMutex sync.Mutex
}

func NewServer(kubeClient *kubernetes.Clientset) *Server {
	return &Server{
		KubeClient: kubeClient,
	}
}

func Run(ctx context.Context, kubeClient *kubernetes.Clientset) error {
	ctrl.Log.Info("Starting static pod manager", "component", "static pod manager")

	apiServer := NewServer(kubeClient)
	http.HandleFunc("/staticpod/create", apiServer.CreateStaticPodHandler)
	http.HandleFunc("/staticpod/delete", apiServer.DeleteStaticPodHandler)
	server := &http.Server{Addr: "127.0.0.1:4576"}

	go func() {
		<-ctx.Done()
		ctrl.Log.Info("Shutting down API server")
		if err := server.Shutdown(ctx); err != nil {
			ctrl.Log.Error(err, "Error shutting down API server")
		}
	}()

	ctrl.Log.Info("Starting API server on :4576")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start API server: %w", err)
	}

	return nil
}

func (s *Server) CreateStaticPodHandler(w http.ResponseWriter, r *http.Request) {
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
		ctrl.Log.Error(err, "Error decoding request body", "component", "static pod manager")
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

	if err := data.validate(); err != nil {
		ctrl.Log.Error(err, "Validation error", "component", "static pod manager")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctrl.Log.Info("Received request to create static pod from: %s, data: %v", r.RemoteAddr, data)

	// Process the templates with the given data and create the static pod and configuration files

	anyFileChanged := false

	changed, err := data.processTemplate(authTemplatePath, authConfigPath, &data.ConfigHashes.AuthTemplateHash)
	if err != nil {
		ctrl.Log.Error(err, "Error processing auth template", "component", "static pod manager")
		http.Error(w, "Error processing auth template", http.StatusInternalServerError)
		return
	}
	anyFileChanged = anyFileChanged || changed

	changed, err = data.processTemplate(distributionTemplatePath, distributionConfigPath, &data.ConfigHashes.DistributionTemplateHash)
	if err != nil {
		ctrl.Log.Error(err, "Error processing distribution template", "component", "static pod manager")
		http.Error(w, "Error processing distribution template", http.StatusInternalServerError)
		return
	}
	anyFileChanged = anyFileChanged || changed

	changed, err = data.processTemplate(staticPodTemplatePath, staticPodConfigPath, nil)
	if err != nil {
		ctrl.Log.Error(err, "Error processing static pod template", "component", "static pod manager")
		http.Error(w, "Error processing static pod template", http.StatusInternalServerError)
		return
	}
	anyFileChanged = anyFileChanged || changed

	if anyFileChanged {
		ctrl.Log.Info("Static pod and configuration created/updated successfully", "component", "static pod manager")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "Static pod and configuration created/updated successfully"})
	} else {
		ctrl.Log.Info("No changes in static pod and configuration", "component", "static pod manager")
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) DeleteStaticPodHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		ctrl.Log.Info("method not allowed", "component", "static pod manager", "endpoint", r.RequestURI, "method", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lock the requestMutex to prevent concurrent requests and release it after the request is processed
	s.requestMutex.Lock()
	defer s.requestMutex.Unlock()

	anyFileDeleted := false

	// Delete the static pod file
	deleted, err := deleteFile(staticPodConfigPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting static pod file", "component", "static pod manager")
		http.Error(w, "Error deleting static pod file", http.StatusInternalServerError)
		return
	}
	anyFileDeleted = anyFileDeleted || deleted

	// Delete the auth config file
	deleted, err = deleteFile(authConfigPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting auth config file", "component", "static pod manager")
		http.Error(w, "Error deleting auth config file", http.StatusInternalServerError)
		return
	}
	anyFileDeleted = anyFileDeleted || deleted

	// Delete the distribution config file
	deleted, err = deleteFile(distributionConfigPath)
	if err != nil {
		ctrl.Log.Error(err, "Error deleting distribution config file", "component", "static pod manager")
		http.Error(w, "Error deleting distribution config file", http.StatusInternalServerError)
		return
	}
	anyFileDeleted = anyFileDeleted || deleted

	// Return 204 if no file was deleted, otherwise 200
	if anyFileDeleted {
		ctrl.Log.Info("Static pod and configuration deleted successfully", "component", "static pod manager")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "Static pod and configuration deleted successfully"})
	} else {
		ctrl.Log.Info("No static pod and configuration to delete", "component", "static pod manager")
		w.WriteHeader(http.StatusNoContent)
	}
}
