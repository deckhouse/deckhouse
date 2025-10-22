---
title: "Модуль local-path-provisioner: FAQ"
---

## Как настроить Prometheus на использование локального хранилища?

Применить custom resource `LocalPathProvisioner`:

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

- `spec.nodeGroups` должен совпадать с NodeGroup, где запущен под Prometheus’а.
- `spec.path` - путь на узле, где будут лежать данные.

Добавить в конфигурацию модуля `prometheus` следующие параметры:

```yaml
longtermStorageClass: localpath-system
storageClass: localpath-system
```

Дождаться переката подов Prometheus.
