/*
Copyright 2024 Flant JSC

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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

type Discoverer struct {
	logger *log.Entry
	client kubernetes.Interface
}

type Config struct {
	KubeconfigDataBase64 string `json:"kubeconfigDataBase64"`
	Namespace            string `json:"namespace"`
}

func parseEnvToConfig() (*Config, error) {
	c := &Config{}
	kubeconfigDataBase64 := os.Getenv("KUBECONFIG_DATA_BASE64")
	if kubeconfigDataBase64 == "" {
		return nil, fmt.Errorf("KUBECONFIG_DATA_BASE64 env should be set")
	}
	c.KubeconfigDataBase64 = kubeconfigDataBase64

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		return nil, fmt.Errorf("NAMESPACE env should be set")
	}
	c.Namespace = namespace
	return c, nil
}

// Client Creates kubeclient
func getClient(KubeconfigDataBase64 string) (*kubernetes.Clientset, error) {
	kubeconfigData, err := base64.StdEncoding.DecodeString(KubeconfigDataBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig data: %v", err.Error())
	}

	config, err := clientcmd.NewClientConfigFromBytes(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("building kube client config: %v", err.Error())
	}

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config: %v", err.Error())
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes client: %v", err.Error())
	}
	return client, err
}

func NewDiscoverer(logger *log.Entry) *Discoverer {
	config, err := parseEnvToConfig()
	if err != nil {
		logger.Fatalf("Cannot get opts from env: %v", err)
	}

	client, err := getClient(config.KubeconfigDataBase64)
	if err != nil {
		logger.Fatalf("Failed to create kubernetes client: %v", err)
	}

	return &Discoverer{
		logger: logger,
		client: client,
	}
}

func (d *Discoverer) DiscoveryData(_ context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	discoveryData := &v1alpha1.DVPCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	storageClasses := make([]v1alpha1.DVPStorageClass, 0)
	scList, err := d.client.StorageV1().StorageClasses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list storage classes: %v", err)
	}
	for _, sc := range scList.Items {
		scdata := &v1alpha1.DVPStorageClass{}
		scdata.Name = sc.GetName()

		scAnnotations := sc.GetAnnotations()
		if scAnnotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			scdata.IsDefault = true
		}

		storageClasses = append(storageClasses, *scdata)
	}

	discoveryData.StorageClasses = storageClasses

	discoveryDataJson, err := json.Marshal(discoveryData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal discovery data: %v", err)
	}

	d.logger.Debugf("discovery data: %v", discoveryDataJson)
	return discoveryDataJson, nil
}

// NotImplemented
func (d *Discoverer) InstanceTypes(_ context.Context) ([]v1alpha1.InstanceType, error) {
	return []v1alpha1.InstanceType{}, nil
}

// NotImplemented
func (d *Discoverer) DisksMeta(ctx context.Context) ([]v1alpha1.DiskMeta, error) {
	return []v1alpha1.DiskMeta{}, nil
}
