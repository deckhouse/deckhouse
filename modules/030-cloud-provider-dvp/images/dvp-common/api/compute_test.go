/*
Copyright 2026 Flant JSC

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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
)

func makeVMBDA(diskName, vmHostname string, terminating bool) *v1alpha2.VirtualMachineBlockDeviceAttachment {
	vmbda := &v1alpha2.VirtualMachineBlockDeviceAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vmbda-" + diskName + "-" + vmHostname,
			Namespace: testNamespace,
			Labels: map[string]string{
				attachmentDiskNameLabel:    diskName,
				attachmentMachineNameLabel: vmHostname,
			},
		},
	}
	if terminating {
		vmbda.Finalizers = []string{"virtualization.deckhouse.io/vmbda-protection"}
		vmbda.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	return vmbda
}

func makeAttachedVMD(name string, vmNames ...string) *v1alpha2.VirtualDisk {
	vd := makeVMD(name, v1alpha2.DiskReady, nil)
	for _, vmName := range vmNames {
		vd.Status.AttachedToVirtualMachines = append(vd.Status.AttachedToVirtualMachines, v1alpha2.AttachedVirtualMachine{Name: vmName, Mounted: true})
	}
	return vd
}

func TestWaitDiskDetaching(t *testing.T) {
	const (
		disk     = "pvc-1"
		hostname = "worker-0"
		vmName   = "cluster-worker-0"
	)

	tests := []struct {
		name     string
		objs     []ctrlclient.Object
		detached bool
	}{
		{
			name: "attachment still exists",
			objs: []ctrlclient.Object{makeVMBDA(disk, hostname, false), makeAttachedVMD(disk, vmName)},
		},
		{
			name: "attachment is terminating",
			objs: []ctrlclient.Object{makeVMBDA(disk, hostname, true), makeAttachedVMD(disk)},
		},
		{
			name: "attachment is gone but the disk still lists the VM",
			objs: []ctrlclient.Object{makeAttachedVMD(disk, vmName)},
		},
		{
			name:     "disk is released",
			objs:     []ctrlclient.Object{makeAttachedVMD(disk)},
			detached: true,
		},
		{
			name:     "disk is already given to another VM",
			objs:     []ctrlclient.Object{makeVMBDA(disk, "worker-1", false), makeAttachedVMD(disk, "cluster-worker-1")},
			detached: true,
		},
		{
			name:     "disk is deleted",
			detached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(tt.objs...).Build()
			compute := NewComputeService(&Service{client: c, namespace: testNamespace})

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err := compute.WaitDiskDetaching(ctx, disk, hostname, vmName)

			if tt.detached && err != nil {
				t.Fatalf("expected the disk to be detached, got %v", err)
			}
			if !tt.detached && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected to keep waiting until the deadline, got %v", err)
			}
		})
	}
}
