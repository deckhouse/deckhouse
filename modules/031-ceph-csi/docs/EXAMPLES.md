---
title: "The ceph-csi module: examples"
---

## An example of the `CephCSIDriver` configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CephCSIDriver
metadata:
  name: example
spec:
  clusterID: 2bf085fc-5119-404f-bb19-820ca6a1b07e
  monitors:
  - 10.0.0.10:6789
  userID: admin
  userKey: AQDbc7phl+eeGRAAaWL9y71mnUiRHKRFOWMPCQ==
  rbd:
    storageClasses:
    - allowVolumeExpansion: true
      defaultFSType: ext4
      mountOptions:
      - discard
      namePostfix: csi-rbd
      pool: kubernetes-rbd
      reclaimPolicy: Delete
  cephfs:
    storageClasses:
    - allowVolumeExpansion: true
      fsName: cephfs
      namePostfix: csi-cephfs
      pool: cephfs_data
      reclaimPolicy: Delete
```
