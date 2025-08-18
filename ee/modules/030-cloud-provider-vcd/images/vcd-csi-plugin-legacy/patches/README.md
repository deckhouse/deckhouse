# Patches

## 001-add-iops-calculation.patch

Files:

- pkg/vcdcsiclient/disks.go
- go.sum
- go.mod

Changes:

- Added IOPS calculation on disk create in the case of iops limits are enabled. Upstream [patch](https://github.com/vmware/cloud-director-named-disk-csi-driver/pull/241).

## 002-remove_pod_mount_path_when_NodeUnpublishVolume_is_called.patch

Files:

- pkg/csi/node.go

Changes:

- https://github.com/vmware/cloud-director-named-disk-csi-driver/commit/cfe7981efa821611c24e9973d547d156374cb586
- https://github.com/kubernetes/kubernetes/issues/122342

## 003-go-mod.patch

Files:

- go.mod
- go.sum

Changes:

- Update dependencies

## 004-metadata.patch

Files:

- cmd/csi/main.go
- pkg/csi/controller.go
- pkg/vcdcsiclient/disks.go
- pkg/csi/driver.go

Changes:

- Add ability to read structured metadata from file and add it to the VCD named disks.
