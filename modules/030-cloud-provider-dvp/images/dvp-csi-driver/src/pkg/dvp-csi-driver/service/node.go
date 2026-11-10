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
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	mount "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"

	dvpapi "dvp-common/api"

	"dvp-csi-driver/pkg/utils"
)

type NodeService struct {
	csi.UnimplementedNodeServer
	nodeName    string
	dvpCloudAPI *dvpapi.DVPCloudAPI
	volumeLocks *volumeLocks

	mounter           mount.Interface
	exec              utilexec.Interface
	getDevicePathFunc func(context.Context, string) (string, error)
}

const (
	// vmBlockDeviceLimit is the DVP limit on block devices per VM (VMBlockDeviceAttachedLimit in virtualization).
	vmBlockDeviceLimit = 16
	// fallbackMaxVolumesPerNode leaves room for the system disk when the node VM cannot be read.
	fallbackMaxVolumesPerNode = vmBlockDeviceLimit - 1
)

var NodeCaps = []csi.NodeServiceCapability_RPC_Type{
	csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
	csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
	csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
}

func NewNode(
	nodeName string,
	dvpCloudAPI *dvpapi.DVPCloudAPI,
) *NodeService {
	return &NodeService{
		nodeName:    nodeName,
		dvpCloudAPI: dvpCloudAPI,
		volumeLocks: newVolumeLocks(),

		mounter: mount.New(""),
		exec:    utilexec.New(),
	}
}

func (n *NodeService) NodeStageVolume(
	ctx context.Context,
	req *csi.NodeStageVolumeRequest,
) (*csi.NodeStageVolumeResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id not provided")
	}
	diskName := req.VolumeId

	klog.Infof("Staging volume %v", diskName)

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability not provided")
	}
	if volumeCapability.GetBlock() != nil {
		klog.Infof("Volume %v is a block volume, no need for staging", diskName)
		return &csi.NodeStageVolumeResponse{}, nil
	}
	mountCapability := volumeCapability.GetMount()
	if mountCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "mount volume capability not provided")
	}
	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path not provided")
	}

	lockKey := volumeLockKey(diskName, "stage")
	if !n.volumeLocks.tryAcquire(lockKey) {
		return nil, status.Errorf(codes.Aborted, "volume %q staging already in progress", diskName)
	}
	defer n.volumeLocks.release(lockKey)

	device, err := n.resolveDevicePath(ctx, diskName)
	if err != nil {
		klog.Errorf("Failed to fetch device by for volume %v", diskName)
		return nil, err
	}

	if err := os.MkdirAll(stagingTargetPath, 0o750); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create staging target path %s: %v", stagingTargetPath, err)
	}

	isMountPoint, err := n.mounter.IsMountPoint(stagingTargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check staging target path %s mount point: %v", stagingTargetPath, err)
	}
	if isMountPoint {
		klog.Infof("Staging target path %s is already mounted, skipping stage", stagingTargetPath)
		return &csi.NodeStageVolumeResponse{}, nil
	}

	fsType := mountCapability.GetFsType()
	mounter := mount.NewSafeFormatAndMount(n.mounter, n.exec)
	if err := mounter.FormatAndMountSensitive(device, stagingTargetPath, fsType, nil, mountCapability.GetMountFlags()); err != nil {
		return nil, stageMountStatusError(device, stagingTargetPath, fsType, err)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func stageMountStatusError(source, target, fsType string, err error) error {
	code := codes.Internal
	var mountErr mount.MountError
	if errors.As(err, &mountErr) {
		switch mountErr.Type {
		case mount.FilesystemMismatch, mount.UnformattedReadOnly, mount.HasFilesystemErrors:
			code = codes.FailedPrecondition
		case mount.GetDiskFormatFailed, mount.FormatFailed, mount.UnknownMountError:
			code = codes.Internal
		}
	}

	return status.Errorf(code, "failed to stage mount source %s to target %s with fs type %q: %v", source, target, fsType, err)
}

func (n *NodeService) NodeUnstageVolume(_ context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	diskName := req.GetVolumeId()
	if diskName == "" {
		return nil, status.Error(codes.InvalidArgument, "volume id not provided")
	}
	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path not provided")
	}

	lockKey := volumeLockKey(diskName, "stage")
	if !n.volumeLocks.tryAcquire(lockKey) {
		return nil, status.Errorf(codes.Aborted, "volume %q staging already in progress", diskName)
	}
	defer n.volumeLocks.release(lockKey)

	if err := mount.CleanupMountPoint(stagingTargetPath, n.mounter, false); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to cleanup staging target path %s: %v", stagingTargetPath, err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (n *NodeService) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest,
) (*csi.NodePublishVolumeResponse, error) {
	if len(req.VolumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id not provided")
	}
	diskName := req.VolumeId

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path not provided")
	}

	lockKey := volumeLockKey(diskName, targetPath)
	if !n.volumeLocks.tryAcquire(lockKey) {
		return nil, status.Errorf(codes.Aborted, "volume %q operation already in progress for %q", diskName, targetPath)
	}
	defer n.volumeLocks.release(lockKey)

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capability not provided")
	}

	if volumeCapability.GetBlock() != nil {
		device, err := n.resolveDevicePath(ctx, diskName)
		if err != nil {
			klog.Errorf("Failed to fetch device by for volume %v", diskName)
			return nil, err
		}

		return n.publishBlockVolume(req, device)
	}
	if volumeCapability.GetMount() == nil {
		return nil, status.Error(codes.InvalidArgument, "mount volume capability not provided")
	}
	stagingTargetPath := req.GetStagingTargetPath()
	if stagingTargetPath == "" {
		return nil, status.Error(codes.FailedPrecondition, "staging target path not provided")
	}

	err := os.MkdirAll(targetPath, 0o750)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create target path %s: %v", targetPath, err)
	}

	isMountPoint, err := n.mounter.IsMountPoint(targetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check target path %s mount point: %v", targetPath, err)
	}
	if isMountPoint {
		klog.Infof("Filesystem target path %s is already mounted, skipping publish", targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}
	if err := n.ensureFilesystemStaged(ctx, req); err != nil {
		return nil, err
	}

	options := []string{"bind"}
	if req.GetReadonly() {
		options = append(options, "ro")
	}

	klog.Infof("Bind mounting staged path %s on target path %s", stagingTargetPath, targetPath)

	err = n.mounter.Mount(stagingTargetPath, targetPath, "", options)
	if err != nil {
		klog.Errorf("Failed mounting %v", err)
		return nil, status.Errorf(codes.Internal, "failed to bind mount staged path %s to target path %s: %v", stagingTargetPath, targetPath, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (n *NodeService) ensureFilesystemStaged(ctx context.Context, req *csi.NodePublishVolumeRequest) error {
	stagingTargetPath := req.GetStagingTargetPath()
	isMountPoint, err := n.mounter.IsMountPoint(stagingTargetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return status.Errorf(codes.Internal, "failed to check staging target path %s mount point: %v", stagingTargetPath, err)
		}
		isMountPoint = false
	}
	if isMountPoint {
		return nil
	}

	_, err = n.NodeStageVolume(ctx, &csi.NodeStageVolumeRequest{
		VolumeId:          req.GetVolumeId(),
		PublishContext:    req.GetPublishContext(),
		StagingTargetPath: stagingTargetPath,
		VolumeCapability:  req.GetVolumeCapability(),
		Secrets:           req.GetSecrets(),
		VolumeContext:     req.GetVolumeContext(),
	})
	if err != nil {
		return err
	}

	isMountPoint, err = n.mounter.IsMountPoint(stagingTargetPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to check staging target path %s mount point after staging: %v", stagingTargetPath, err)
	}
	if !isMountPoint {
		return status.Errorf(codes.FailedPrecondition, "staging target path %s is not mounted after staging", stagingTargetPath)
	}

	return nil
}

func (n *NodeService) resolveDevicePath(ctx context.Context, diskName string) (string, error) {
	if n.getDevicePathFunc != nil {
		return n.getDevicePathFunc(ctx, diskName)
	}
	return n.getDevicePath(ctx, diskName)
}

func (n *NodeService) getDevicePath(ctx context.Context, diskName string) (string, error) {
	disk, err := n.dvpCloudAPI.DiskService.GetDiskByName(ctx, diskName)
	if err != nil {
		msg := fmt.Errorf("error from parent DVP cluster while finding disk %v by id, error: %v", diskName, err)
		klog.Error(msg.Error())
		return "", msg
	}

	if len(disk.Status.AttachedToVirtualMachines) > 1 {
		msg := fmt.Errorf("disk %v has more than one compute node, can't find device path", diskName)
		klog.Error(msg.Error())
		return "", msg
	}
	if len(disk.Status.AttachedToVirtualMachines) == 0 {
		msg := fmt.Errorf("disk %v has no compute node, can't find device path", diskName)
		klog.Error(msg.Error())
		return "", msg
	}

	hash := md5.Sum([]byte(disk.UID))
	target := hex.EncodeToString(hash[:])

	device := fmt.Sprintf("/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_%s", target)
	_, err = os.Stat(device)
	if err != nil {
		msg := fmt.Errorf("device path %s for disk ID %v does not exists", device, diskName)
		klog.Errorf("%s", msg.Error())
		return "", msg
	}

	klog.Infof("Device path %s exists", device)
	return device, nil
}

func (n *NodeService) publishBlockVolume(req *csi.NodePublishVolumeRequest, device string) (*csi.NodePublishVolumeResponse, error) {
	file, err := os.OpenFile(req.TargetPath, os.O_CREATE, os.FileMode(0o644))
	if err != nil {
		if !os.IsExist(err) {
			return nil, status.Errorf(codes.Internal, "Failed to create targetPath %s, err: %v", req.TargetPath, err)
		}
	}
	defer func() {
		if file == nil {
			return
		}
		if closeErr := file.Close(); closeErr != nil {
			klog.Errorf("Failed to close file %s, err: %v", req.TargetPath, closeErr)
		}
	}()

	isMountPoint, err := n.mounter.IsMountPoint(req.TargetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, status.Errorf(codes.Internal, "failed to check mount point %s: %v", req.TargetPath, err)
		}

		isMountPoint = false
	}
	if isMountPoint {
		klog.Infof("Block target path %s is already mounted, skipping publish", req.TargetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	klog.Infof("Publishing block volume, device: %s, req: %+v", device, req)

	err = n.mounter.Mount(device, req.TargetPath, "", []string{"bind"})
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
	target := req.GetTargetPath()
	if target == "" {
		return nil, status.Error(codes.InvalidArgument, "targetpath not provided")
	}

	lockKey := volumeLockKey(req.GetVolumeId(), target)
	if !n.volumeLocks.tryAcquire(lockKey) {
		return nil, status.Errorf(codes.Aborted, "volume %q operation already in progress for %q", req.GetVolumeId(), target)
	}
	defer n.volumeLocks.release(lockKey)

	klog.Infof("NodeUnpublishVolume: unmounting %s", target)

	err := mount.CleanupMountPoint(target, n.mounter, true)
	if err == nil || strings.Contains(fmt.Sprint(err), "not mounted") {
		klog.Infof("NodeUnpublishVolume: target %s already unmounted (or not a mount), ignoring: %v", target, err)
	} else {
		return nil, status.Errorf(codes.Internal, "failed to unmount target %s: %v", target, err)
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
	switch {
	case strings.HasPrefix(fsType, "ext"):
		resizeCmd = "resize2fs"
	case strings.HasPrefix(fsType, "xfs"):
		resizeCmd = "xfs_growfs"
	default:
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

func (n *NodeService) NodeGetInfo(ctx context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	maxVolumes := int64(fallbackMaxVolumesPerNode)
	vm, err := n.dvpCloudAPI.ComputeService.GetVMByHostname(ctx, n.nodeName)
	if err != nil {
		// Failing here would keep the plugin unregistered on the node, so a limit that may be off by a few disks is better.
		klog.Warningf("Failed to get VM %v from parent DVP cluster, reporting %d volumes per node: %v", n.nodeName, maxVolumes, err)
	} else {
		maxVolumes = maxVolumesPerNode(len(vm.Spec.BlockDeviceRefs))
		klog.Infof("VM %v has %d static block devices, reporting %d volumes per node", n.nodeName, len(vm.Spec.BlockDeviceRefs), maxVolumes)
	}

	return &csi.NodeGetInfoResponse{NodeId: n.nodeName, MaxVolumesPerNode: maxVolumes}, nil
}

// maxVolumesPerNode returns how many volumes can be hotplugged to a VM with the given number of static block devices.
func maxVolumesPerNode(staticDisks int) int64 {
	// Zero means no limit in CSI, so a VM without free slots still reports one.
	return int64(max(vmBlockDeviceLimit-staticDisks, 1))
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
