---
title: "Модуль local-path-provisioner: FAQ"
---

## Как настроить Prometheus на использование локального хранилища?

Применить CR `LocalPathProvisioner`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

- `spec.nodeGroups` должен совпадать с нодгруппой prometheus'ов.
- `spec.path` - путь на узле где будут лежать данные.

Добавить в конфигурацию Deckhouse (configMap `d8-system/deckhouse`):

```yaml
prometheus: |
  longtermStorageClass: localpath-system
  storageClass: localpath-system
```

Дождаться переката Pod'ов Prometheus.
