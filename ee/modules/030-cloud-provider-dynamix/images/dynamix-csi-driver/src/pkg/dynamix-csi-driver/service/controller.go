/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	dynamixapi "dynamix-common/api"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/disks"
)

const (
	ParameterPool      = "pool"
	ParameterAccountID = "accountId"
	ParameterGID       = "gId"
	ParameterSEPID     = "sepId"
)

type ControllerService struct {
	csi.UnimplementedControllerServer
	dynamixCloudAPI *dynamixapi.DynamixCloudAPI
}

var ControllerCaps = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME, // attach/detach
	csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
}

func NewController(
	dynamixCloudAPI *dynamixapi.DynamixCloudAPI,
) *ControllerService {
	return &ControllerService{
		dynamixCloudAPI: dynamixCloudAPI,
	}
}

func parseParameters(params map[string]string) (string, uint64, uint64, uint64, error) {
	pool := params[ParameterPool]

	accountID, err := strconv.ParseUint(params[ParameterAccountID], 10, 64)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("can't parse accountId: %v in StorageClass parameters, %w ", params[ParameterAccountID], err)
	}

	gID, err := strconv.ParseUint(params[ParameterGID], 10, 64)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("can't parse gId: %v in StorageClass parameters, %w ", params[ParameterGID], err)
	}

	sepID, err := strconv.ParseUint(params[ParameterSEPID], 10, 64)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("can't parse sepId: %v in StorageClass parameters, %w ", params[ParameterSEPID], err)
	}

	return pool, accountID, gID, sepID, nil
}

func (c *ControllerService) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest,
) (*csi.CreateVolumeResponse, error) {
	klog.Infof("Creating disk %s", req.Name)
	pool, accountID, gID, sepID, err := parseParameters(req.Parameters)
	if err != nil {
		return nil, fmt.Errorf("error parse storageClass paramater %w", err)
	}

	if len(pool) == 0 {
		return nil, fmt.Errorf("error required storageClass paramater %s wasn't set",
			ParameterPool)
	}
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
	disks, err := c.dynamixCloudAPI.DiskService.ListDisksByName(ctx, diskName)
	if err != nil {
		msg := fmt.Errorf("error while finding disk %s by name, error: %w", diskName, err)
		klog.Errorf(msg.Error())
		return nil, msg
	}
	if len(disks) > 1 {
		msg := fmt.Errorf(
			"found more then one disk with the name %s,"+
				"please contanct the Dynamix admin to check the name duplication", diskName)
		klog.Errorf(msg.Error())
		return nil, msg
	}

	result := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{},
	}

	if len(disks) == 1 {
		disk := disks[0]
		result.Volume.VolumeId = strconv.FormatUint(disk.ID, 10)
		result.Volume.CapacityBytes = int64(disk.SizeMax)
		return result, nil
	}

	disk, err := c.dynamixCloudAPI.DiskService.CreateDisk(
		ctx,
		accountID,
		gID,
		diskName,
		convertBytesToGigabytes(uint64(requiredSize)),
		pool,
		sepID,
	)
	if err != nil {
		msg := fmt.Errorf("error while creating disk %s, error: %w", diskName, err)
		klog.Errorf(msg.Error())
		return nil, msg
	}

	result.Volume.VolumeId = strconv.FormatUint(disk.ID, 10)
	result.Volume.CapacityBytes = int64(convertGigabytesToBytes(disk.SizeMax))
	return result, nil
}

func (c *ControllerService) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest,
) (*csi.DeleteVolumeResponse, error) {
	diskID, err := strconv.ParseUint(req.VolumeId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error required paramater VolumeId can't parse: %w", err)
	}
	klog.Infof("Removing disk %v", diskID)

	_, err = c.dynamixCloudAPI.DiskService.GetDisk(ctx, diskID)
	if err != nil {
		if errors.Is(err, dynamixapi.ErrNotFound) {
			return &csi.DeleteVolumeResponse{}, nil
		}
		msg := fmt.Errorf("error while finding disk %v by id, error: %w", diskID, err)
		klog.Errorf(msg.Error())
		return nil, msg
	}

	err = c.dynamixCloudAPI.DiskService.RemoveDisk(ctx, diskID)
	if err != nil {
		msg := fmt.Errorf("failed removing disk %v by id, error: %w", diskID, err)
		klog.Errorf(msg.Error())
		return nil, msg
	}

	klog.Infof("Finished removing disk %v", diskID)
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

	diskID, err := strconv.ParseUint(req.VolumeId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error required paramater VolumeId can't parse: %w", err)
	}

	vm, err := c.dynamixCloudAPI.ComputeSvc.GetVMByName(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("failed finding VM: %v, error: %w", req.NodeId, err)
	}

	computeID := vm.ID

	attached, err := c.hasDiskAttachedToVM(ctx, diskID, computeID)
	if err != nil {
		klog.Errorf(err.Error())
		return nil, err
	}

	if attached {
		klog.Infof("Disk %v is already attached to VM %v, returning OK", diskID, computeID)
		return &csi.ControllerPublishVolumeResponse{}, nil
	}

	err = c.dynamixCloudAPI.ComputeSvc.AttachDiskToVM(ctx, diskID, computeID)
	if err != nil {
		msg := fmt.Errorf("failed creating disk attachment, error: %w", err)
		klog.Errorf(msg.Error())
		return nil, msg
	}

	klog.Infof("Attached Disk %v to VM %v", diskID, computeID)
	return &csi.ControllerPublishVolumeResponse{}, nil
}
func (c *ControllerService) hasDiskAttachedToVM(
	ctx context.Context,
	diskID uint64,
	computeID uint64,
) (bool, error) {
	vm, err := c.dynamixCloudAPI.ComputeSvc.GetVMByID(ctx, computeID)
	if err != nil {
		return false, fmt.Errorf("failed finding VM: %v, error: %w", computeID, err)
	}

	for _, disk := range vm.Disks {
		if disk.ID == diskID {
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

	diskID, err := strconv.ParseUint(req.VolumeId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error required paramater VolumeId can't parse: %w", err)
	}

	vm, err := c.dynamixCloudAPI.ComputeSvc.GetVMByName(ctx, req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("failed finding VM: %v, error: %w", req.NodeId, err)
	}

	computeID := vm.ID

	attached, err := c.hasDiskAttachedToVM(ctx, diskID, computeID)
	if err != nil {
		klog.Errorf(err.Error())
		return nil, err
	}

	if !attached {
		klog.Infof("Disk attachment %v for VM %v already detached, returning OK", diskID, computeID)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	err = c.dynamixCloudAPI.ComputeSvc.DetachDiskFromVM(ctx, diskID, computeID)
	if err != nil {
		msg := fmt.Errorf("failed removing disk %v from VM %v, error: %w", diskID, computeID, err)
		klog.Errorf(msg.Error())
		return nil, msg
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (c *ControllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID not provided")
	}

	volumeID, err := strconv.ParseUint(req.GetVolumeId(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error required paramater VolumeId can't parse: %w", err)
	}

	capRange := req.GetCapacityRange()
	if capRange == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range not provided")
	}
	newSizeBytes := capRange.GetRequiredBytes()

	if newSizeBytes < 0 {
		return nil, status.Error(codes.InvalidArgument, "Required Bytes cannot be negative")
	}

	newSize := convertBytesToGigabytes(uint64(newSizeBytes))

	klog.Infof("Expanding volume %v to %v Gb.", volumeID, newSize)
	disk, err := c.dynamixCloudAPI.DiskService.GetDisk(ctx, volumeID)
	if err != nil {
		if errors.Is(err, dynamixapi.ErrNotFound) {
			msg := fmt.Errorf("disk %v wasn't found", volumeID)
			klog.Error(msg)
			return nil, status.Error(codes.NotFound, msg.Error())
		}
		msg := fmt.Errorf("error while finding disk %v, error: %w", volumeID, err)
		klog.Error(msg)
		return nil, status.Error(codes.Internal, msg.Error())
	}

	diskSize := disk.SizeMax
	if diskSize >= newSize {
		klog.Infof("Volume %v of size %d is larger than requested size %d, no need to extend",
			volumeID, diskSize, newSize)
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         int64(diskSize),
			NodeExpansionRequired: false,
		}, nil
	}

	err = c.dynamixCloudAPI.DiskService.Resize2Disk(
		ctx,
		volumeID,
		newSize,
	)
	if err != nil {
		return nil, status.Errorf(codes.ResourceExhausted, "failed to expand volume %v: %w", volumeID, err)
	}
	klog.Infof("Expanded Disk %v to %v Gb", volumeID, newSize)

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         int64(convertGigabytesToBytes(newSize)),
		NodeExpansionRequired: isNodeExpansionRequired(req.GetVolumeCapability(), disk),
	}, nil
}

func isNodeExpansionRequired(
	vc *csi.VolumeCapability,
	disk *disks.ItemDisk,
) bool {
	// If this is a raw block device, no expansion should be necessary on the node
	if vc != nil && vc.GetBlock() != nil {
		return false
	}
	// If disk is not attached to any VM then no need to expand
	if len(disk.Computes) == 0 {
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
