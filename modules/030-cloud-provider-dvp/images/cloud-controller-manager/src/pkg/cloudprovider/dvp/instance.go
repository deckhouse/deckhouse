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

package dvp

import (
	"context"
	"dvp-common/api"
	"errors"
	"fmt"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

func (c *Cloud) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	vm, err := c.getVMByNodeName(ctx, nodeName)
	if err != nil {
		return nil, err
	}

	return c.extractNodeAddressesFromVM(vm)
}

func (c *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	vm, err := c.getVMByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return c.extractNodeAddressesFromVM(vm)
}

func (c *Cloud) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	vm, err := c.getVMByNodeName(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return vm.Name, nil
}

func (c *Cloud) InstanceType(_ context.Context, _ types.NodeName) (string, error) {
	return "", nil
}

func (c *Cloud) InstanceTypeByProviderID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *Cloud) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

func (c *Cloud) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

func (c *Cloud) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	_, err := c.getVMByProviderID(ctx, providerID)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (c *Cloud) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	vm, err := c.getVMByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return vm.Status.Phase == v1alpha2.MachineStopped, nil
}

func (c *Cloud) getVMByNodeName(ctx context.Context, nodeName types.NodeName) (*v1alpha2.VirtualMachine, error) {
	vmHostname := MapNodeNameToVMName(nodeName)

	var vm *v1alpha2.VirtualMachine

	vm, err := c.dvpService.ComputeService.GetVMByHostname(ctx, vmHostname)
	if err != nil {
		vm, err = c.dvpService.ComputeService.GetVMByName(ctx, vmHostname)
		if err != nil && errors.Is(err, api.ErrNotFound) {
			return nil, cloudprovider.InstanceNotFound
		} else if err != nil {
			return nil, err
		}
	}
	return vm, nil
}

func (c *Cloud) getVMByProviderID(ctx context.Context, providerID string) (*v1alpha2.VirtualMachine, error) {
	vmName, err := ParseProviderID(providerID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compute id [%s]: %v", vmName, err)
	}

	vm, err := c.dvpService.ComputeService.GetVMByName(ctx, vmName)
	if err != nil && errors.Is(err, api.ErrNotFound) {
		return nil, cloudprovider.InstanceNotFound
	} else if err != nil {
		return nil, err
	}

	return vm, err
}

func (c *Cloud) extractNodeAddressesFromVM(vm *v1alpha2.VirtualMachine) ([]v1.NodeAddress, error) {
	var nodeAddress []v1.NodeAddress

	externalIP, localIP, err := c.dvpService.ComputeService.GetVMIPAddresses(vm)
	if err != nil {
		return nil, err
	}

	for _, ip := range externalIP {
		nodeAddress = append(nodeAddress, v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: ip,
		})
	}

	if len(localIP) == 0 {
		localIP = externalIP
	}

	for _, ip := range localIP {
		nodeAddress = append(nodeAddress, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: ip,
		})
	}

	hostname, err := c.dvpService.ComputeService.GetVMHostname(vm)
	if err == nil {
		nodeAddress = append(nodeAddress, v1.NodeAddress{
			Type:    v1.NodeHostName,
			Address: hostname,
		})
	}

	return nodeAddress, nil
}
