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
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
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
	DVPLoadBalancerLabelPrefix = "dvp.deckhouse.io/"
)

type ComputeService struct {
	*Service
}

func NewComputeService(service *Service) *ComputeService {
	return &ComputeService{service}
}

func (c *ComputeService) CreateVM(ctx context.Context, machine *v1alpha2.VirtualMachine) (*v1alpha2.VirtualMachine, error) {
	if machine.Namespace != "" && machine.Namespace != c.namespace {
		return nil, fmt.Errorf("namespace mismatch: expected %s got %s", c.namespace, machine.ObjectMeta.Namespace)
	}

	machine.Namespace = c.namespace

	if err := c.client.Create(ctx, machine); err != nil {
		return nil, fmt.Errorf("create VirtualMachine resource: %w", err)
	}

	err := c.Wait(ctx, machine.Name, machine, func(obj client.Object) (bool, error) {
		if obj == nil {
			return false, nil
		}

		vm, ok := obj.(*v1alpha2.VirtualMachine)
		if !ok {
			return false, fmt.Errorf("expected a VirtualMachine but got a %T", obj)
		}

		expectedPhase := v1alpha2.MachineRunning
		runPolicy := vm.Spec.RunPolicy
		if runPolicy == v1alpha2.AlwaysOffPolicy || runPolicy == v1alpha2.ManualPolicy {
			expectedPhase = v1alpha2.MachineStopped
		}

		return vm.Status.Phase == expectedPhase, nil
	})
	if err != nil {
		return nil, fmt.Errorf("await VirtualMachine creation: %w", err)
	}

	return machine, nil
}

func (c *ComputeService) GetVMByName(ctx context.Context, name string) (*v1alpha2.VirtualMachine, error) {
	var instance v1alpha2.VirtualMachine

	if err := c.client.Get(ctx, types.NamespacedName{Name: name, Namespace: c.namespace}, &instance); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, cloudprovider.InstanceNotFound
		}
		return nil, err
	}
	return &instance, nil
}

func (c *ComputeService) GetVMByHostname(ctx context.Context, hostname string) (*v1alpha2.VirtualMachine, error) {
	var instanceList v1alpha2.VirtualMachineList

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", DVPVMHostnameLabel, hostname))
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
		return nil, fmt.Errorf("found more than one VM with name %s", hostname)
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

func (c *ComputeService) DeleteVM(ctx context.Context, name string) error {
	vm, err := c.GetVMByName(ctx, name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get VirtualMachine resource: %w", err)
	}

	if err = c.client.Delete(ctx, vm); err != nil {
		return fmt.Errorf("delete VirtualMachine resource: %w", err)
	}

	err = c.Wait(ctx, name, vm, func(obj client.Object) (bool, error) { return obj == nil, nil })
	if err != nil {
		return fmt.Errorf("await VirtualMachine deletion: %w", err)
	}

	return nil
}

func (c *ComputeService) GetDisksForDetachAndDelete(ctx context.Context, vm *v1alpha2.VirtualMachine, detachDisks bool) ([]string, []string, error) {
	disksToDetach := make([]string, 0)
	disksToDelete := make([]string, 0)
	vmHostname, err := c.GetVMHostname(vm)

	vmbdas, err := c.listVMBDAByHostname(ctx, vmHostname)
	if err != nil {
		return nil, nil, err
	}

	vmbdasMap := make(map[string]struct{})

	for _, vdbda := range vmbdas {
		vmbdasMap[vdbda.Spec.BlockDeviceRef.Name] = struct{}{}
	}

	for _, device := range vm.Status.BlockDeviceRefs {
		if !device.Attached || device.Kind != v1alpha2.DiskDevice {
			continue
		}
		if _, ok := vmbdasMap[device.Name]; !ok || !detachDisks {
			disksToDelete = append(disksToDelete, device.Name)
			continue
		}

		disksToDetach = append(disksToDetach, device.Name)
	}

	return disksToDetach, disksToDelete, nil
}

func (c *ComputeService) StartVM(ctx context.Context, name string) error {
	err := c.client.Create(ctx, &v1alpha2.VirtualMachineOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
		},
		Spec: v1alpha2.VirtualMachineOperationSpec{
			Type:           v1alpha2.VMOPTypeStart,
			VirtualMachine: name,
		},
	})
	if err != nil {
		return fmt.Errorf("create VirtualMachineOperation resource: %w", err)
	}

	err = c.Wait(ctx, name, &v1alpha2.VirtualMachine{}, func(obj client.Object) (bool, error) {
		if obj == nil {
			return false, nil
		}
		return obj.(*v1alpha2.VirtualMachine).Status.Phase == v1alpha2.MachineRunning, nil
	})
	if err != nil {
		return fmt.Errorf("wait for VM running phase: %w", err)
	}

	return nil
}

func (c *ComputeService) StopVM(ctx context.Context, name string) error {
	err := c.client.Create(ctx, &v1alpha2.VirtualMachineOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
		},
		Spec: v1alpha2.VirtualMachineOperationSpec{
			Type:           v1alpha2.VMOPTypeStop,
			VirtualMachine: name,
		},
	})
	if err != nil {
		return fmt.Errorf("create VirtualMachineOperation resource: %w", err)
	}

	err = c.Wait(ctx, name, &v1alpha2.VirtualMachine{}, func(obj client.Object) (bool, error) {
		if obj == nil {
			return false, nil
		}
		return obj.(*v1alpha2.VirtualMachine).Status.Phase == v1alpha2.MachineStopped, nil
	})
	if err != nil {
		return fmt.Errorf("wait for VM starting phase: %w", err)
	}

	return nil
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

func (c *ComputeService) AttachDiskToVM(ctx context.Context, diskName string, vmHostname string) error {
	vm, err := c.GetVMByHostname(ctx, vmHostname)
	if err != nil {
		return err
	}

	vmbda, err := c.getVMBDA(ctx, diskName, vmHostname)
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
			Name:      fmt.Sprintf("vmbda-%s-%s", diskName, vmHostname),
			Namespace: c.namespace,
			Labels: map[string]string{
				attachmentDiskNameLabel:    diskName,
				attachmentMachineNameLabel: vmHostname,
			},
		},
		Spec: v1alpha2.VirtualMachineBlockDeviceAttachmentSpec{
			VirtualMachineName: vm.Name,
			BlockDeviceRef: v1alpha2.VMBDAObjectRef{
				Kind: v1alpha2.VMBDAObjectRefKindVirtualDisk,
				Name: diskName,
			},
		},
	}

	err = c.client.Create(ctx, vmbda)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (c *ComputeService) DetachDiskFromVM(ctx context.Context, diskName string, vmName string) error {
	vmbda, err := c.getVMBDA(ctx, diskName, vmName)
	if err != nil {
		return err
	}

	err = c.client.Delete(ctx, vmbda)
	if err != nil {
		return err
	}
	return nil
}

func (c *ComputeService) DetachDisksFromVM(ctx context.Context, disksName []string, vmName string) error {
	merr := &multierror.Error{}
	for _, diskName := range disksName {
		err := c.DetachDiskFromVM(ctx, diskName, vmName)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("detach VirtualDisk %s: %w", diskName, err))
		}
	}
	if err := merr.ErrorOrNil(); err != nil {
		return fmt.Errorf("detach VirtualDisks: %w", err)
	}
	return nil
}

func (c *ComputeService) getVMBDA(ctx context.Context, diskName string, vmHostname string) (*v1alpha2.VirtualMachineBlockDeviceAttachment, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s,%s=%s", attachmentDiskNameLabel, diskName, attachmentMachineNameLabel, vmHostname))
	if err != nil {
		return nil, err
	}

	var vmbdas v1alpha2.VirtualMachineBlockDeviceAttachmentList
	err = c.client.List(ctx, &vmbdas, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     c.namespace,
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

func (c *ComputeService) listVMBDAByHostname(ctx context.Context, vmHostname string) ([]v1alpha2.VirtualMachineBlockDeviceAttachment, error) {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", attachmentMachineNameLabel, vmHostname))
	if err != nil {
		return nil, err
	}

	var vmbdas v1alpha2.VirtualMachineBlockDeviceAttachmentList
	err = c.client.List(ctx, &vmbdas, &client.ListOptions{
		LabelSelector: selector,
		Namespace:     c.namespace,
	})
	if err != nil {
		return nil, err
	}

	return vmbdas.Items, nil
}

func (c *ComputeService) CreateCloudInitProvisioningSecret(ctx context.Context, name string, userData []byte) error {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.namespace,
		},
		Type:       v1alpha2.SecretTypeCloudInit,
		StringData: map[string]string{"userData": string(userData)},
	}

	if _, err := c.clientset.CoreV1().Secrets(c.namespace).Create(ctx, s, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create '%s[%s]' secret: %w", name, v1alpha2.SecretTypeCloudInit, err)
	}
	return nil
}

func (c *ComputeService) DeleteCloudInitProvisioningSecret(ctx context.Context, name string) error {
	if err := c.clientset.CoreV1().Secrets(c.namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("delete '%s[%s]' secret: %w", name, v1alpha2.SecretTypeCloudInit, err)
	}
	return nil
}

func (c *ComputeService) EnsureVMLabelByHostname(ctx context.Context, hostname, key, val string) error {
	vm, err := c.GetVMByHostname(ctx, hostname)
	if err != nil {
		return err
	}

	before := vm.DeepCopy()
	if vm.Labels == nil {
		vm.Labels = map[string]string{}
	}
	if cur, ok := vm.Labels[key]; ok && cur == val {
		return nil
	}
	vm.Labels[key] = val
	return c.client.Patch(ctx, vm, client.MergeFrom(before))
}

func (c *ComputeService) RemoveVMLabelByHostname(ctx context.Context, hostname, key string) error {
	vm, err := c.GetVMByHostname(ctx, hostname)
	if err != nil {
		return err
	}
	if vm.Labels == nil {
		return nil
	}
	if _, ok := vm.Labels[key]; !ok {
		return nil
	}
	before := vm.DeepCopy()
	delete(vm.Labels, key)
	return c.client.Patch(ctx, vm, client.MergeFrom(before))
}
