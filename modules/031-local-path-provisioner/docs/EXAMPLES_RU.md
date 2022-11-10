---
title: "Модуль local-path-provisioner: примеры"
---

## Пример CR `LocalPathProvisioner`

Reclaim policy устанавливается по умолчанию в `Retain`.

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

## Пример CR `LocalPathProvisioner` с установленным `reclaimPolicy`

Reclaim policy устанавливается в `Delete`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
  reclaimPolicy: "Delete"
```
