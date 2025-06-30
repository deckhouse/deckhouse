/*
Copyright 2025 Flant JSC

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

package api

import (
	"errors"

	"dvp-common/config"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrDuplicateAttachment = errors.New("duplicate attachment")
)

type DVPCloudAPI struct {
	Service             *Service
	ComputeService      *ComputeService
	DiskService         *DiskService
	PortalService       *PortalService
	LoadBalancerService *LoadBalancerService
}

func NewDVPCloudAPI(config *config.CloudConfig) (*DVPCloudAPI, error) {
	clientConfig, err := config.GetKubernetesClientConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()
	err = v1alpha2.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	client, err := client.New(clientConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	service := &Service{
		clientset: clientset,
		client:    client,
		namespace: config.Namespace,
	}

	return &DVPCloudAPI{
		Service:             service,
		ComputeService:      NewComputeService(service),
		DiskService:         NewDiskService(service),
		PortalService:       NewPortalService(service),
		LoadBalancerService: NewLoadBalancerService(service),
	}, nil
}

// ProjectNamespace returns the project that this DVPCloudAPI instance is bound to.
func (a *DVPCloudAPI) ProjectNamespace() string {
	return a.Service.namespace
}
