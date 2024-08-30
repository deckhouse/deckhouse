/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	dynamixapi "dynamix-common/api"
	"dynamix-csi-driver/pkg/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/utils/mount"
)

const (
	deviceNamePrefix = "virtio-pci-0000:00:"
)

type NodeService struct {
	csi.UnimplementedNodeServer
	nodeName        string
	dynamixCloudAPI *dynamixapi.DynamixCloudAPI
}

var NodeCaps = []csi.NodeServiceCapability_RPC_Type{
	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
}

func NewNode(
	nodeName string,
	dynamixCloudAPI *dynamixapi.DynamixCloudAPI,
) *NodeService {
	return &NodeService{
		nodeName:        nodeName,
		dynamixCloudAPI: dynamixCloudAPI,
	}
}

func (n *NodeService) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest,
) (*csi.NodeStageVolumeResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, fmt.Errorf("error required request paramater VolumeId wasn't set")
	}
	diskID, err := strconv.ParseUint(req.VolumeId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error required paramater VolumeId can't parse: %w", err)
	}

	klog.Infof("Staging volume %v with %+v", diskID, req)

	if req.VolumeCapability.GetBlock() != nil {
		klog.Infof("Volume %v is a block volume, no need for staging", diskID)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	device, err := n.getDevicePath(ctx, diskID)
	if err != nil {
		klog.Errorf("Failed to fetch device by for volume %v", diskID)
		return nil, err
	}

	// is there a filesystem on this device?
	filesystem, err := utils.GetDeviceInfo(device)
	if err != nil {
		klog.Errorf("Failed to fetch device info for volume %v", diskID)
		return nil, err
	}
	if filesystem != "" {
		klog.Infof("Detected fs %s, returning", filesystem)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	fsType := req.VolumeCapability.GetMount().FsType
	// no filesystem - create it
	klog.Infof("Creating FS %s on device %s", fsType, device)
	err = utils.MakeFS(device, fsType)
	if err != nil {
		klog.Errorf("Could not create filesystem %s on %s", fsType, device)
		return nil, err
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (n *NodeService) NodeUnstageVolume(_ context.Context, _ *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (n *NodeService) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest,
) (*csi.NodePublishVolumeResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, fmt.Errorf("error required request paramater VolumeId wasn't set")
	}
	diskID, err := strconv.ParseUint(req.VolumeId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error required paramater VolumeId can't parse: %w", err)
	}

	device, err := n.getDevicePath(ctx, diskID)
	if err != nil {
		klog.Errorf("Failed to fetch device by for volume %v", diskID)
		return nil, err
	}

	if req.VolumeCapability.GetBlock() != nil {
		return n.publishBlockVolume(req, device)
	}
	targetPath := req.GetTargetPath()
	err = os.MkdirAll(targetPath, 0644)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	fsType := req.VolumeCapability.GetMount().FsType
	klog.Infof("Mounting devicePath %s, on targetPath: %s with FS type: %s",
		device, targetPath, fsType)
	mounter := mount.New("")
	err = mounter.Mount(device, targetPath, fsType, []string{})
	if err != nil {
		klog.Errorf("Failed mounting %v", err)
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *NodeService) getDevicePath(ctx context.Context, diskID uint64) (string, error) {
	disk, err := n.dynamixCloudAPI.DiskService.GetDisk(ctx, diskID)
	if err != nil {
		msg := fmt.Errorf("error while finding disk %v by id, error: %w", diskID, err)
		klog.Errorf(msg.Error())
		return "", msg
	}

	device := fmt.Sprintf("/dev/disk/by-path/%s%.2d.0", deviceNamePrefix, disk.PCISlot)
	_, err = os.Stat(device)
	if err != nil {
		klog.Errorf("Device path for disk ID %v does not exists", diskID)
		return "", errors.New("device was not found")
	}

	klog.Infof("Device path %s exists", device)
	return device, nil

}

func (n *NodeService) publishBlockVolume(req *csi.NodePublishVolumeRequest, device string) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof("Publishing block volume, device: %s, req: %+v", device, req)
	file, err := os.OpenFile(req.TargetPath, os.O_CREATE, os.FileMode(0644))
	defer func() {
		err = file.Close()
		if err != nil {
			klog.Errorf("Failed to close file %s, err: %v", req.TargetPath, err)
		}
	}()
	if err != nil {
		if !os.IsExist(err) {
			return nil, status.Errorf(codes.Internal, "Failed to create targetPath %s, err: %v", req.TargetPath, err)
		}
	}

	mounter := mount.New("")
	err = mounter.Mount(device, req.TargetPath, "", []string{"bind"})
	if err != nil {
		if removeErr := os.Remove(req.TargetPath); removeErr != nil {
			return nil, status.Errorf(codes.Internal, "Failed to remove mount target %v, err: %v, mount error: %v", req.TargetPath, removeErr, err)
		}

		return nil, status.Errorf(codes.Internal, "Failed to mount %v at %v, err: %v", device, req.TargetPath, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *NodeService) NodeUnpublishVolume(
	_ context.Context,
	req *csi.NodeUnpublishVolumeRequest,
) (*csi.NodeUnpublishVolumeResponse, error) {
	mounter := mount.New("")
	klog.Infof("Unmounting %s", req.GetTargetPath())
	err := mounter.Unmount(req.GetTargetPath())
	if err != nil {
		klog.Infof("Failed to unmount")
		return nil, err
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (n *NodeService) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume ID was empty")
	}

	if len(req.VolumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats volume path was empty")
	}

	_, err := os.Lstat(req.VolumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "Path %s does not exist", req.VolumePath)
		}
		return nil, status.Errorf(codes.Internal, "Unknown error when getting stats on %s: %v", req.VolumePath, err)
	}

	isBlock, err := utils.IsBlockDevice(req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to determine whether %s is block device: %v", req.VolumePath, err)
	}

	// If volume is a block device, return only size in bytes.
	if isBlock {
		bcap, err := utils.GetBlockSizeBytes(req.VolumePath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to get block size on path %s: %v", req.VolumePath, err)
		}
		return &csi.NodeGetVolumeStatsResponse{
			Usage: []*csi.VolumeUsage{
				{
					Unit:  csi.VolumeUsage_BYTES,
					Total: bcap,
				},
			},
		}, nil
	}

	// We assume filesystem presence on volume as raw block device is ruled out and try to get fs stats
	available, capacity, used, inodesFree, inodes, inodesUsed, err := utils.StatFS(req.VolumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get fs info on path %s: %v", req.VolumePath, err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Unit:      csi.VolumeUsage_BYTES,
				Available: available,
				Total:     capacity,
				Used:      used,
			},
			{
				Unit:      csi.VolumeUsage_INODES,
				Available: inodesFree,
				Total:     inodes,
				Used:      inodesUsed,
			},
		},
	}, nil
}

func (n *NodeService) NodeExpandVolume(_ context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume path must be provided")
	}
	volumeCapability := req.GetVolumeCapability()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capability must be provided")
	}
	var resizeCmd string
	fsType := volumeCapability.GetMount().FsType
	if strings.HasPrefix(fsType, "ext") {
		resizeCmd = "resize2fs"
	} else if strings.HasPrefix(fsType, "xfs") {
		resizeCmd = "xfs_growfs"
	} else {
		return nil, status.Error(codes.InvalidArgument, "fsType is neither xfs or ext[234]")
	}
	klog.Infof("Resizing filesystem %s mounted on %s with %s", fsType, volumePath, resizeCmd)

	device, err := utils.GetDeviceByMountPoint(volumePath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(resizeCmd, device)
	err = cmd.Run()
	var exitError *exec.ExitError
	if err != nil && errors.As(err, &exitError) {
		return nil, status.Error(codes.Internal, err.Error()+" resize failed with "+exitError.Error())
	}

	klog.Infof("Resized %s filesystem on device %s)", fsType, device)
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (n *NodeService) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{NodeId: n.nodeName}, nil
}

func (n *NodeService) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	caps := make([]*csi.NodeServiceCapability, 0, len(NodeCaps))
	for _, c := range NodeCaps {
		caps = append(
			caps,
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: c,
					},
				},
			},
		)
	}
	return &csi.NodeGetCapabilitiesResponse{Capabilities: caps}, nil
}
