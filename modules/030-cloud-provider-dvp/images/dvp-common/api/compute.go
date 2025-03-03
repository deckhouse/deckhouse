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
	"errors"
	"fmt"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DVPVMHostnameLabel         = "dvp.deckhouse.io/hostname"
	attachmentDiskNameLabel    = "virtualMachineDiskName"
	attachmentMachineNameLabel = "virtualMachineName"
)

type ComputeService struct {
	*Service
}

func NewComputeService(service *Service) *ComputeService {
	return &ComputeService{service}
}

func (c *ComputeService) GetVMByName(ctx context.Context, name string) (*v1alpha2.VirtualMachine, error) {
	var instanceList v1alpha2.VirtualMachineList

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", DVPVMHostnameLabel, name))
	if err != nil {
		return nil, err
	}

	opts := &client.ListOptions{
		Namespace:     c.namespace,
		LabelSelector: selector,
	}

	if err := c.client.List(ctx, &instanceList, opts); err != nil {
		if k8serrors.IsNotFound(err) {
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
	if err := c.client.List(ctx, &instanceList, &client.ListOptions{Namespace: c.namespace}); err != nil {
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

func (d *ComputeService) AttachDiskToVM(ctx context.Context, diskName string, computeName string) error {
	vmbda, err := d.getVMBDA(ctx, diskName, computeName)
	if vmbda != nil && err == nil {
		return nil
	}

	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}

	vmbda = &v1alpha2.VirtualMachineBlockDeviceAttachment{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha2.VirtualMachineBlockDeviceAttachmentKind,
			APIVersion: v1alpha2.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("vmbda-%s-%s", diskName, computeName),
			Namespace: d.namespace,
			Labels: map[string]string{
				attachmentDiskNameLabel:    diskName,
				attachmentMachineNameLabel: computeName,
			},
		},
		Spec: v1alpha2.VirtualMachineBlockDeviceAttachmentSpec{
			VirtualMachineName: computeName,
			BlockDeviceRef: v1alpha2.VMBDAObjectRef{
				Kind: v1alpha2.VMBDAObjectRefKindVirtualDisk,
				Name: diskName,
			},
		},
	}

	err = d.client.Create(ctx, vmbda)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (d *ComputeService) DetachDiskFromVM(ctx context.Context, diskName string, computeName string) error {
	vmbda, err := d.getVMBDA(ctx, diskName, computeName)
	if err != nil {
		return err
	}

	err = d.client.Delete(ctx, vmbda)
	if err != nil {
		return err
	}
	return nil
}

func (d *ComputeService) getVMBDA(ctx context.Context, diskName, computeName string) (*v1alpha2.VirtualMachineBlockDeviceAttachment, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s,%s=%s", attachmentDiskNameLabel, diskName, attachmentMachineNameLabel, computeName))
	if err != nil {
		return nil, err
	}

	var vmbdas v1alpha2.VirtualMachineBlockDeviceAttachmentList
	err = d.client.List(ctx, &vmbdas, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     d.namespace,
	})
	if err != nil {
		return nil, err
	}

	if len(vmbdas.Items) == 0 {
		return nil, ErrNotFound
	}

	if len(vmbdas.Items) != 1 {
		return nil, fmt.Errorf("more attachments found than expected: please report a bug %w", ErrDuplicateAttachment)
	}

	return &vmbdas.Items[0], nil
}
