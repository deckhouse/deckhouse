/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package zvirt

import (
	"context"
	"errors"

	"github.com/deckhouse/zvirt-cloud-controller-manager/pkg/zvirtapi"
	ovirtclient "github.com/ovirt/go-ovirt-client/v3"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

func (zc *Cloud) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	vm, err := zc.getVMByNodeName(ctx, nodeName)
	if err != nil {
		return nil, err
	}

	return zc.extractNodeAddressesFromVM(ctx, vm)
}

func (zc *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	vm, err := zc.getVMByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return zc.extractNodeAddressesFromVM(ctx, vm)
}

func (zc *Cloud) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	vm, err := zc.getVMByNodeName(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return string(vm.ID()), nil
}

func (zc *Cloud) InstanceType(_ context.Context, _ types.NodeName) (string, error) {
	return "", nil
}

func (zc *Cloud) InstanceTypeByProviderID(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (zc *Cloud) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

func (zc *Cloud) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

func (zc *Cloud) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	_, err := zc.getVMByProviderID(ctx, providerID)
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (zc *Cloud) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	vm, err := zc.getVMByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}
	return vm.Status() == ovirtclient.VMStatusDown, nil
}

func (zc *Cloud) getVMByNodeName(ctx context.Context, nodename types.NodeName) (ovirtclient.VM, error) {
	vmName := MapNodeNameToVMName(nodename)

	vm, err := zc.zvirtService.ComputeSvc.GetVMByName(ctx, vmName)
	if err != nil && errors.Is(err, zvirtapi.ErrNotFound) {
		return nil, cloudprovider.InstanceNotFound
	} else if err != nil {
		return nil, err
	}

	return vm, nil
}

func (zc *Cloud) getVMByProviderID(ctx context.Context, providerID string) (ovirtclient.VM, error) {
	vmID, err := ParseProviderID(providerID)
	if err != nil {
		return nil, err
	}

	vm, err := zc.zvirtService.ComputeSvc.GetVMByID(ctx, vmID)
	if err != nil && errors.Is(err, zvirtapi.ErrNotFound) {
		return nil, cloudprovider.InstanceNotFound
	} else if err != nil {
		return nil, err
	}

	return vm, err
}

func (zc *Cloud) extractNodeAddressesFromVM(ctx context.Context, vm ovirtclient.VM) ([]v1.NodeAddress, error) {
	nodeAddress := []v1.NodeAddress{}

	externalIP, localIP, err := zc.zvirtService.ComputeSvc.GetVMIPAddresses(ctx, vm)
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
		Address: vm.Name(),
	})

	return nodeAddress, nil
}
