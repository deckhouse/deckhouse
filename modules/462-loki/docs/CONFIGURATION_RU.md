---
title: "Модуль loki: настройки"
---

<!-- SCHEMA -->

## Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    effectiveStorageClass: ceph-csi-rbd
    diskSizeGigabytes: 10
    retentionPeriod: 48h
  version: 1
```
