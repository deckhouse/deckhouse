package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"deckhouse.io/uibackend/cloudprovider"
	"github.com/julienschmidt/httprouter"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type discoveryData struct {
	Paths []string `json:"paths"`

	KubernetesVersion string                 `json:"kubernetesVersion"`
	CloudProvider     map[string]interface{} `json:"cloudProvider,omitempty"`
}

type discoveryCollector struct {
	cs   *kubernetes.Clientset
	data *discoveryData
}

func newDiscoveryCollector(cs *kubernetes.Clientset) *discoveryCollector {
	return &discoveryCollector{
		cs: cs,
		data: &discoveryData{
			Paths:         make([]string, 0),
			CloudProvider: make(map[string]interface{}),
		},
	}
}

func (dc *discoveryCollector) AddPath(p string) {
	dc.data.Paths = append(dc.data.Paths, p)
}

func (dc *discoveryCollector) SetKubeVersion(v string) {
	dc.data.KubernetesVersion = v
}

func (dc *discoveryCollector) AddCloudProvider(ctx context.Context, cloudProviderName string) error {
	dc.data.CloudProvider["name"] = cloudProviderName
	providerData, err := cloudprovider.Discover(ctx, cloudProviderName, dc.cs)
	if err != nil {
		return err
	}
	for k, v := range providerData {
		dc.data.CloudProvider[k] = v
	}
	return nil
}

func (dc *discoveryCollector) Build() *discoveryData {
	// sort.Strings(dc.data.Paths) // or let it be in the order of appending?
	return dc.data
}

func handleDiscovery(clientset *kubernetes.Clientset, discovery *discoveryData) httprouter.Handle {
	lock := sync.Mutex{}
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// The version of the Kubernetes API server can change, so we need to check it every time
		kubeVersion, err := clientset.ServerVersion()
		if err != nil {
			klog.Errorf("failed to get kube version: %v", err)
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(err.Error()))
			return
		}
		v := kubeVersion.String()

		if discovery.KubernetesVersion != v {
			lock.Lock()
			discovery.KubernetesVersion = v
			lock.Unlock()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	}
}
