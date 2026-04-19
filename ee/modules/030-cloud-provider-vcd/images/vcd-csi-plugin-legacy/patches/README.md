# Patches

## 001-add-iops-calculation.patch

Files:

- pkg/vcdcsiclient/disks.go

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

- Update and pin dependency versions required to fix known CVEs.
- The `go.mod` dependency updates were moved here from `001-add-iops-calculation.patch` to avoid patch ordering conflicts.
- `001-add-iops-calculation.patch` now contains only code changes, while dependency updates are isolated in this patch.

## 004-metadata.patch

Files:

- cmd/csi/main.go
- pkg/csi/controller.go
- pkg/vcdcsiclient/disks.go
- pkg/csi/driver.go

Changes:

- Add ability to read structured metadata from file and add it to the VCD named disks.
