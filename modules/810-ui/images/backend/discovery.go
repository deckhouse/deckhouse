package main

import (
	"context"
	"sort"

	"deckhouse.io/uibackend/cloudprovider"
	"k8s.io/client-go/kubernetes"
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
	sort.Strings(dc.data.Paths)
	return dc.data
}
