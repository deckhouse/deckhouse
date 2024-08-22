/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package dynamix

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/compute"

	"dynamix-common/api"
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

	return strconv.FormatUint(vm.ID, 10), nil
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

	return vm.TechStatus == "STOPPED", nil
}

func (c *Cloud) getVMByNodeName(ctx context.Context, nodeName types.NodeName) (*compute.ItemCompute, error) {
	vmName := MapNodeNameToVMName(nodeName)

	vm, err := c.dynamixService.ComputeSvc.GetVMByName(ctx, vmName)
	if err != nil && errors.Is(err, api.ErrNotFound) {
		return nil, cloudprovider.InstanceNotFound
	} else if err != nil {
		return nil, err
	}

	return vm, nil
}

func (c *Cloud) getVMByProviderID(ctx context.Context, providerID string) (*compute.ItemCompute, error) {
	vmID, err := ParseProviderID(providerID)
	if err != nil {
		return nil, err
	}
	computeID, err := strconv.ParseUint(vmID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compute id [%s]: %v", vmID, err)
	}

	vm, err := c.dynamixService.ComputeSvc.GetVMByID(ctx, computeID)
	if err != nil && errors.Is(err, api.ErrNotFound) {
		return nil, cloudprovider.InstanceNotFound
	} else if err != nil {
		return nil, err
	}

	return vm, err
}

func (c *Cloud) extractNodeAddressesFromVM(vm *compute.ItemCompute) ([]v1.NodeAddress, error) {
	var nodeAddress []v1.NodeAddress

	externalIP, localIP, err := c.dynamixService.ComputeSvc.GetVMIPAddresses(vm)
	if err != nil {
		return nil, err
	}

	for _, ip := range externalIP {
		nodeAddress = append(nodeAddress, v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: ip,
		})
	}

	for _, ip := range localIP {
		nodeAddress = append(nodeAddress, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: ip,
		})
	}

	nodeAddress = append(nodeAddress, v1.NodeAddress{
		Type:    v1.NodeHostName,
		Address: vm.Name,
	})

	return nodeAddress, nil
}
