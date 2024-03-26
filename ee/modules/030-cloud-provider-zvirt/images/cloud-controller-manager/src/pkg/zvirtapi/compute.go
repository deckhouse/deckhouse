/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package zvirtapi

import (
	"context"
	"fmt"

	ovirtclient "github.com/ovirt/go-ovirt-client/v3"
	"k8s.io/klog/v2"
)

type ComputeService struct {
	client ovirtclient.ClientWithLegacySupport
}

func NewComputeService(client ovirtclient.ClientWithLegacySupport) *ComputeService {
	return &ComputeService{
		client: client,
	}
}

func (cSvc *ComputeService) GetVMByName(ctx context.Context, name string) (ovirtclient.VM, error) {
	vm, err := cSvc.client.GetVMByName(name, getRetryStrategy(ctx)...)
	if err != nil && ovirtclient.HasErrorCode(err, ovirtclient.ENotFound) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	} else if err != nil {
		return nil, err
	}
	return vm, nil
}

func (cSvc *ComputeService) GetVMByID(ctx context.Context, id string) (ovirtclient.VM, error) {
	vm, err := cSvc.client.GetVM(ovirtclient.VMID(id))
	if err != nil && ovirtclient.HasErrorCode(err, ovirtclient.ENotFound) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	} else if err != nil {
		return nil, err
	}

	return vm, nil
}

// GetVMIPAddresses return external and local IPv4
func (cSvc *ComputeService) GetVMIPAddresses(ctx context.Context, vm ovirtclient.VM) ([]string, []string, error) {
	externalIPMap := make(map[string]struct{})
	externalIPs := []string{}
	localIPs := []string{}

	vmExternalIPs, err := vm.GetNonLocalIPAddresses(getRetryStrategy(ctx)...)
	if err != nil {
		return nil, nil, err
	}
	for _, ips := range vmExternalIPs {
		for _, ip := range ips {
			// skip IPv6
			if ip.To4() == nil {
				klog.V(4).Infof("GetVMIPAddresses: externalIP [%v] skipped, not IPv4", ip.String())
				continue
			}

			strIP := ip.String()
			externalIPMap[strIP] = struct{}{}
			externalIPs = append(externalIPs, strIP)
			klog.V(4).Infof("GetVMIPAddresses: externalIP [%v] ", strIP)
		}
	}

	vmIPs, err := vm.GetIPAddresses(ovirtclient.NewVMIPSearchParams(), getRetryStrategy(ctx)...)
	if err != nil {
		return nil, nil, err
	}

	for _, ips := range vmIPs {
		for _, ip := range ips {
			// skip IPv6
			if ip.To4() == nil {
				klog.V(4).Infof("GetVMIPAddresses: ip [%v] skipped, not IPv4", ip.String())
				continue
			}

			strIP := ip.String()

			// skip external IP
			if _, ok := externalIPMap[strIP]; ok {
				klog.V(4).Infof("GetVMIPAddresses: ip [%v] skipped, externalIP", strIP)
				continue
			}
			localIPs = append(localIPs, strIP)
			klog.V(4).Infof("GetVMIPAddresses: localIP [%v] ", strIP)
		}
	}

	return externalIPs, localIPs, nil
}

func (cSvc *ComputeService) GetVMHostName(ctx context.Context, vm ovirtclient.VM) string {
	hostname := vm.Name()
	return hostname
}
