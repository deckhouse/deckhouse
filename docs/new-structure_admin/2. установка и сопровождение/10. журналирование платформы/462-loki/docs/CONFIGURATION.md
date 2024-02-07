---
title: "The loki module: configuration"
---

<!-- SCHEMA -->

## An example of the configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    storageClass: ceph-csi-rbd
    diskSizeGigabytes: 10
    retentionPeriodHours: 48
  enabled: true
  version: 1
```
