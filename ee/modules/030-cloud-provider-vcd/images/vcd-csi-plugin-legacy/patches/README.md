# Patches

## Add IOPS calculation

Added IOPS calculation on disk create in the case of iops limits are enabled. Upstream [patch](https://github.com/vmware/cloud-director-named-disk-csi-driver/pull/241).


## remove pod mount path when NodeUnpublishVolume is called
https://github.com/vmware/cloud-director-named-disk-csi-driver/commit/cfe7981efa821611c24e9973d547d156374cb586
https://github.com/kubernetes/kubernetes/issues/122342