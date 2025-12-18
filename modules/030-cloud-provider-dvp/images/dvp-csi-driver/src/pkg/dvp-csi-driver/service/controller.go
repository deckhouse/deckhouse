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

package service

import (
	"context"
	"dvp-csi-driver/pkg/utils"
	"errors"
	"fmt"

	dvpapi "dvp-common/api"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	ParameterDVPStorageClass = "dvpStorageClass"
)

type ControllerService struct {
	csi.UnimplementedControllerServer
	dvpCloudAPI *dvpapi.DVPCloudAPI
}

var ControllerCaps = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME, // attach/detach
	csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
}

func NewController(
	dvpCloudAPI *dvpapi.DVPCloudAPI,
) *ControllerService {
	return &ControllerService{
		dvpCloudAPI: dvpCloudAPI,
	}
}

func checkRequiredParams(params map[string]string) error {
	for _, paramName := range []string{ParameterDVPStorageClass} {
		if len(params[paramName]) == 0 {
			return fmt.Errorf("error required storageClass paramater %s wasn't set",
				paramName)
		}
	}
	return nil
}

func (c *ControllerService) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest,
) (*csi.CreateVolumeResponse, error) {
	klog.Infof("Creating disk %s", req.Name)

	if err := checkRequiredParams(req.Parameters); err != nil {
		return nil, err
	}

	dvpStorageClass := req.Parameters[ParameterDVPStorageClass]

	diskName := req.Name
	if len(diskName) == 0 {
		return nil, fmt.Errorf("error required request parameter Name was not provided")
	}

	// Check access mode
	for _, cap := range req.GetVolumeCapabilities() {
		if cap.AccessMode.Mode != csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY &&
			cap.AccessMode.Mode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("unsupported access mode %s, currently only RWO is supported", cap.AccessMode.Mode))
		}
	}
	requiredSize := req.CapacityRange.GetRequiredBytes()

	if requiredSize < 0 {
		return nil, status.Error(codes.InvalidArgument, "Required Bytes cannot be negative")
	}

	// Check if a disk with the same name already exist
	disks, err := c.dvpCloudAPI.DiskService.ListDisksByName(ctx, diskName)
	if err != nil {
		msg := fmt.Errorf("error from parent DVP cluster while finding disk %s by name: %v", diskName, err)
		klog.Error(msg.Error())
		return nil, msg
	}
	if len(disks.Items) > 1 {
		msg := fmt.Errorf(
			"found more then one disk with the name %s,"+
				"please contanct the DVP admin to check the name duplication", diskName)
		klog.Error(msg.Error())
		return nil, msg
	}

	result := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{},
	}

	if len(disks.Items) == 1 {
		disk := disks.Items[0]
		diskCapacity, err := utils.ConvertStringQuantityToInt64(disk.Status.Capacity)
		if err != nil {
			klog.Error(err.Error())
			return nil, err
		}
		result.Volume.VolumeId = disk.Name
		result.Volume.CapacityBytes = diskCapacity
		return result, nil
	}

	disk, err := c.dvpCloudAPI.DiskService.CreateDisk(
		ctx,
		diskName,
		requiredSize,
		dvpStorageClass,
	)
	if err != nil {
		msg := fmt.Errorf("error from parent DVP cluster while creating disk %s: %v", diskName, err)
		klog.Error(msg.Error())
		return nil, msg
	}

	result.Volume.VolumeId = disk.Name
	result.Volume.CapacityBytes = requiredSize
	return result, nil
}

func (c *ControllerService) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest,
) (*csi.DeleteVolumeResponse, error) {
	diskName := req.VolumeId
	klog.Infof("Removing disk %v", diskName)

	_, err := c.dvpCloudAPI.DiskService.GetDiskByName(ctx, diskName)
	if err != nil {
		if errors.Is(err, dvpapi.ErrNotFound) {
			return &csi.DeleteVolumeResponse{}, nil
		}
		msg := fmt.Errorf("error from parent DVP cluster while finding disk %v by id: %v", diskName, err)
		klog.Error(msg.Error())
		return nil, msg
	}

	err = c.dvpCloudAPI.DiskService.RemoveDiskByName(ctx, diskName)
	if err != nil {
		msg := fmt.Errorf("error from parent DVP cluster while removing disk %v by id: %v", diskName, err)
		klog.Error(msg.Error())
		return nil, msg
	}

	klog.Infof("Finished removing disk %v", diskName)
	return &csi.DeleteVolumeResponse{}, nil
}

func (c *ControllerService) ControllerPublishVolume(
	ctx context.Context, req *csi.ControllerPublishVolumeRequest,
) (*csi.ControllerPublishVolumeResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, fmt.Errorf("error required request paramater VolumeId wasn't set")
	}
	if len(req.NodeId) == 0 {
		return nil, fmt.Errorf("error required request paramater NodeId wasn't set")
	}

	diskName := req.VolumeId

	vm, err := c.dvpCloudAPI.ComputeService.GetVMByHostname(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("error from parent DVP cluster while finding VM: %v: %v", req.NodeId, err)
	}

	attached, err := c.hasDiskAttachedToVM(diskName, vm)
	if err != nil {
		klog.Error(err.Error())
		return nil, err
	}

	if attached {
		klog.Infof("Disk %v is already attached to VM %v, returning OK", diskName, req.NodeId)
		return &csi.ControllerPublishVolumeResponse{}, nil
	}

	err = c.dvpCloudAPI.ComputeService.AttachDiskToVM(ctx, diskName, req.NodeId)
	if err != nil {
		msg := fmt.Errorf("error from parent DVP cluster while creating disk attachment: %v", err)
		klog.Error(msg.Error())
		return nil, msg
	}

	klog.Infof("Attached Disk %v to VM %v", diskName, req.NodeId)
	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (c *ControllerService) hasDiskAttachedToVM(
	diskName string,
	vm *v1alpha2.VirtualMachine,
) (bool, error) {
	for _, diskRef := range vm.Status.BlockDeviceRefs {
		if diskRef.Name == diskName {
			return true, nil
		}
	}

	return false, nil
}

func (c *ControllerService) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest,
) (*csi.ControllerUnpublishVolumeResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, fmt.Errorf("error required request paramater VolumeId wasn't set")
	}
	if len(req.NodeId) == 0 {
		return nil, fmt.Errorf("error required request paramater NodeId wasn't set")
	}

	diskName := req.VolumeId

	vm, err := c.dvpCloudAPI.ComputeService.GetVMByHostname(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("error from parent DVP cluster while finding VM: %v: %v", req.NodeId, err)
	}

	vmHostname := req.NodeId

	attached, err := c.hasDiskAttachedToVM(diskName, vm)
	if err != nil {
		klog.Error(err.Error())
		return nil, err
	}

	if !attached {
		klog.Infof("Disk attachment %v for VM %v already detached, returning OK", diskName, vmHostname)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	err = c.dvpCloudAPI.ComputeService.DetachDiskFromVM(ctx, diskName, vmHostname)
	if err != nil {
		msg := fmt.Errorf("error from parent DVP cluster while removing disk %v from VM %v: %v", diskName, vmHostname, err)
		klog.Error(msg.Error())
		return nil, msg
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (c *ControllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volumeName := req.GetVolumeId()

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}
	requestedSizeBytes := capRange.GetRequiredBytes()

	if requestedSizeBytes < 0 {
		return nil, status.Error(codes.InvalidArgument, "Required Bytes cannot be negative")
	}

	newSize := utils.ConvertInt64ToStringQuantity(requestedSizeBytes)

	klog.Infof("Expanding volume %v to %v", volumeName, newSize)
	disk, err := c.dvpCloudAPI.DiskService.GetDiskByName(ctx, volumeName)
	if err != nil {
		if errors.Is(err, dvpapi.ErrNotFound) {
			msg := fmt.Errorf("disk %v wasn't found", volumeName)
			klog.Error(msg)
			return nil, status.Error(codes.NotFound, msg.Error())
		}
		msg := fmt.Errorf("error from parent DVP cluster while finding disk %v: %v", volumeName, err)
		klog.Error(msg)
		return nil, status.Error(codes.Internal, msg.Error())
	}

	diskSize, err := utils.ConvertStringQuantityToInt64(disk.Status.Capacity)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if diskSize >= requestedSizeBytes {
		klog.Infof("Volume %v of size %d is larger than requested size %s, no need to extend",
			volumeName, diskSize, newSize)
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         int64(diskSize),
			NodeExpansionRequired: false,
		}, nil
	}

	err = c.dvpCloudAPI.DiskService.ResizeDisk(
		ctx,
		volumeName,
		newSize,
	)
	if err != nil {
		return nil, status.Errorf(codes.ResourceExhausted, "failed to expand volume %v, error from parent DVP cluster: %v", volumeName, err)
	}
	klog.Infof("Expanded Disk %v to %v", volumeName, newSize)

	newSizeBytes, err := utils.ConvertStringQuantityToInt64(newSize)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         newSizeBytes,
		NodeExpansionRequired: isNodeExpansionRequired(req.GetVolumeCapability(), disk),
	}, nil
}

func isNodeExpansionRequired(
	vc *csi.VolumeCapability,
	disk *v1alpha2.VirtualDisk,
) bool {
	// If this is a raw block device, no expansion should be necessary on the node
	if vc != nil && vc.GetBlock() != nil {
		return false
	}
	// If disk is not attached to any VM then no need to expand
	if (disk != nil && len(disk.Status.AttachedToVirtualMachines) == 0) || disk == nil {
		return false
	}
	return true
}

func (c *ControllerService) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	caps := make([]*csi.ControllerServiceCapability, 0, len(ControllerCaps))
	for _, capability := range ControllerCaps {
		caps = append(
			caps,
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: capability,
					},
				},
			},
		)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}
