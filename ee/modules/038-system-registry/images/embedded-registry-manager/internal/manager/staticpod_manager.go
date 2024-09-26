package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
)

const manifestDir = "/etc/kubernetes/manifests"
const manifestFile = "registry-master.yaml"

type Server struct {
	KubeClient *kubernetes.Clientset
}

func NewServer(kubeClient *kubernetes.Clientset) *Server {
	return &Server{
		KubeClient: kubeClient,
	}
}

func Run(ctx context.Context, kubeClient *kubernetes.Clientset) error {
	ctrl.Log.Info("Starting static pod manager")

	apiServer := NewServer(kubeClient)
	http.HandleFunc("/staticpod/create", apiServer.CreateStaticPodHandler)
	http.HandleFunc("/staticpod/delete", apiServer.DeleteStaticPodHandler)
	server := &http.Server{Addr: "0.0.0.0:4576"}

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

func checkAndDeployStaticPod() error {
	manifestPath := filepath.Join(manifestDir, manifestFile)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return deployStaticPod(manifestPath)
	}
	return nil
}

func deployStaticPod(manifestPath string) error {
	manifest := `
apiVersion: v1
kind: Pod
metadata:
  name: registry-master
  namespace: d8-system
spec:
  containers:
  - name: seaweedfs
    image: chrislusf/seaweedfs:latest
    command: ["weed", "master"]
    ports:
    - containerPort: 9333
  - name: docker-distribution
    image: registry:2
    ports:
    - containerPort: 5000
  - name: docker-auth
    image: cesanta/docker_auth:1
    ports:
    - containerPort: 5001
`

	/*	if err := ioutil.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
			return fmt.Errorf("failed to write manifest: %w", err)
		}
	*/
	ctrl.Log.Info("Deploying static pod manifest", "path", manifestPath)
	ctrl.Log.Info("Creating manifest", "manifest", manifest)

	return nil
}

func (s *Server) CreateStaticPodHandler(w http.ResponseWriter, r *http.Request) {
	err := CreateStaticPod()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Static pod created"})
}

func (s *Server) DeleteStaticPodHandler(w http.ResponseWriter, r *http.Request) {
	err := DeleteStaticPod()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Static pod deleted"})
}

func CreateStaticPod() error {
	manifestPath := filepath.Join(manifestDir, manifestFile)
	return deployStaticPod(manifestPath)
}

func DeleteStaticPod() error {
	manifestPath := filepath.Join(manifestDir, manifestFile)
	/*
		if err := os.Remove(manifestPath); err != nil {
			return fmt.Errorf("failed to delete manifest: %w", err)
		}
	*/
	ctrl.Log.Info("Deleting static pod manifest", "path", manifestPath)
	return nil
}
