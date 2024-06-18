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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
)

var storageProfileGVR = schema.GroupVersionResource{
	Group:    "internal.virtualization.deckhouse.io",
	Version:  "v1beta1",
	Resource: "dvpinternalstorageprofiles",
}

type Discoverer struct {
	logger        *log.Entry
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
}

type Config struct {
	KubeconfigDataBase64 string `json:"kubeconfigDataBase64"`
	Namespace            string `json:"namespace"`
}

type StorageProfile struct {
	Status struct {
		ClaimPropertySets []struct {
			AccessModes []string `json:"accessModes"`
			VolumeMode  string   `json:"volumeMode"`
		} `json:"claimPropertySets"`
		StorageClass string `json:"storageClass"`
	} `json:"status"`
}

func isRWX(am []string) bool {
	var RWX bool
	for _, mode := range am {
		if mode == "ReadWriteMany" {
			RWX = true
		}
	}
	return RWX
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

func NewDiscoverer(logger *log.Entry) (*Discoverer, error) {
	config, err := parseEnvToConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot get opts from env: %v", err)
	}

	kubeconfigData, err := base64.StdEncoding.DecodeString(config.KubeconfigDataBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig data: %v", err.Error())
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("building kube client config: %v", err.Error())
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config: %v", err.Error())
	}

	// client for storage classes
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("building kubernetes client: %v", err.Error())
	}

	// client for dvpinternal storage profiles
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client: %v", err.Error())
	}

	return &Discoverer{
		logger:        logger,
		client:        client,
		dynamicClient: dynamicClient,
	}, nil
}

func (d *Discoverer) DiscoveryData(_ context.Context, cloudProviderDiscoveryData []byte) ([]byte, error) {
	discoveryData := &v1alpha1.DVPCloudProviderDiscoveryData{}
	if len(cloudProviderDiscoveryData) > 0 {
		err := json.Unmarshal(cloudProviderDiscoveryData, &discoveryData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cloud provider discovery data: %v", err)
		}
	}

	storageProfiles, err := d.dynamicClient.Resource(storageProfileGVR).Namespace("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	storageClasses := make([]v1alpha1.DVPStorageClass, 0)

	for _, spRaw := range storageProfiles.Items {
		var sp StorageProfile
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(spRaw.UnstructuredContent(), &sp)

		if err != nil {
			d.logger.Errorf("failed to unmarshal storage profile: %v", err)
			continue
		}

		for _, ClaimPropertySet := range sp.Status.ClaimPropertySets {
			if !(ClaimPropertySet.VolumeMode == "Block" && isRWX(ClaimPropertySet.AccessModes)) {
				continue
			}
			sc, err := d.client.StorageV1().StorageClasses().Get(context.Background(), sp.Status.StorageClass, metav1.GetOptions{})
			if err != nil {
				d.logger.Errorf("failed to get storage class: %v", err)
				continue
			}
			scData := &v1alpha1.DVPStorageClass{}
			scData.Name = sc.GetName()
			scAnnotations := sc.GetAnnotations()
			if scAnnotations["storageclass.kubernetes.io/is-default-class"] == "true" {
				scData.IsDefault = true
			}
			storageClasses = append(storageClasses, *scData)
		}
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
