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
	"context"
	"fmt"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DVPVMHostnameLabel = "dvp.deckhouse.io/hostname"
)

type ComputeService struct {
	*Service
}

func NewComputeService(service *Service) *ComputeService {
	return &ComputeService{service}
}

func (c *ComputeService) GetVMByName(ctx context.Context, name string) (*v1alpha2.VirtualMachine, error) {
	var instanceList v1alpha2.VirtualMachineList

	labels := map[string]string{
		DVPVMHostnameLabel: name,
	}

	opts := []client.ListOption{
		client.InNamespace(c.namespace),
		client.MatchingLabels(labels),
	}

	if err := c.client.List(ctx, &instanceList, opts...); err != nil {
		if errors.IsNotFound(err) {
			return nil, cloudprovider.InstanceNotFound
		}
		return nil, err
	}

	if len(instanceList.Items) == 0 {
		return nil, cloudprovider.InstanceNotFound
	}

	if len(instanceList.Items) > 1 {
		return nil, fmt.Errorf("found more than one VM with name %s", name)
	}

	return &instanceList.Items[0], nil
}

func (c *ComputeService) GetVMByID(ctx context.Context, id string) (*v1alpha2.VirtualMachine, error) {
	var (
		instanceList v1alpha2.VirtualMachineList
		instance     *v1alpha2.VirtualMachine
	)
	if err := c.client.List(ctx, &instanceList, client.InNamespace(c.namespace)); err != nil {
		return nil, err
	}

	for _, obj := range instanceList.Items {
		if obj.GetUID() == types.UID(id) {
			instance = &obj
			break
		}
	}

	if instance == nil {
		return nil, cloudprovider.InstanceNotFound
	}

	return instance, nil
}

func (c *ComputeService) GetVMIPAddresses(vm *v1alpha2.VirtualMachine) ([]string, []string, error) {
	if vm == nil {
		return nil, nil, cloudprovider.InstanceNotFound
	}
	return []string{vm.Status.IPAddress}, []string{vm.Status.IPAddress}, nil
}

func (c *ComputeService) GetVMHostname(vm *v1alpha2.VirtualMachine) (string, error) {
	if vm == nil {
		return "", cloudprovider.InstanceNotFound
	}
	if hostname, ok := vm.Labels[DVPVMHostnameLabel]; ok {
		return hostname, nil
	}
	return "", cloudprovider.InstanceNotFound
}
