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

## Что происходит с существующими томами при изменении `spec.path`?

Новые тома создаются по новому пути. Тома, созданные раньше, остаются в старом каталоге. При `reclaimPolicy: Delete` каталог удалённого тома удаляет вспомогательный под (helper pod), и для этого он монтирует старый путь. Когда Pod Security Standards в системных пространствах имён не только предупреждают о нарушениях, но и отклоняют поды, SecurityPolicyException `helper-<name>` этого LocalPathProvisioner разрешает только текущий путь, поэтому вспомогательный под для старого тома отклоняется: PersistentVolume остаётся в статусе `Released`, а его данные — на узле. При `reclaimPolicy: Retain` (по умолчанию) вспомогательный под при удалении не запускается, и это ограничение не действует.

Перед изменением `spec.path` удалите PersistentVolumeClaim томов, созданных по старому пути. Если после изменения такой том остался в статусе `Released`, удалите его каталог на узле, а затем сам PersistentVolume.
