# Patches

## 001-add-iops-calculation.patch

Files:

- pkg/vcdcsiclient/disks.go

Changes:

- Added IOPS calculation on disk create in the case of iops limits are enabled. Upstream [patch](https://github.com/vmware/cloud-director-named-disk-csi-driver/pull/241).

## 002-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 003-metadata.patch

Files:

- cmd/csi/main.go
- pkg/csi/controller.go
- pkg/vcdcsiclient/disks.go
- pkg/csi/driver.go

Changes:

- Add ability to read structured metadata from file and add it to the VCD named disks.
