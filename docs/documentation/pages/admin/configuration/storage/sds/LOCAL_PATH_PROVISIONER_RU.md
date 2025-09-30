---
title: "Локальное хранилище Local Path Provisioner"
permalink: ru/admin/configuration/storage/sds/local-path-provisioner.html
lang: ru
---

Deckhouse Kubernetes Platform предоставляет возможность настраивать локальные хранилища Local Path Provisioner. Это простое решение без поддержки снимков и ограничений на размер, которое лучше всего подходит для разработки, тестирования и небольших кластеров. Данное хранилище использует локальное дисковое пространство узлов Kubernetes для создания PersistentVolume, не полагаясь на внешние системы хранения данных.

## Принцип работы

Для каждого ресурса [LocalPathProvisioner](/modules/local-path-provisioner/cr.html#localpathprovisioner) создается соответствующий объект StorageClass. Список узлов, на которых разрешено использовать StorageClass, определяется на основе поля `nodeGroups` и используется при размещении подов.

При запросе диска подом происходит следующее:

- создаётся PersistentVolume с типом `HostPath`;
- на нужном узле создается директория, путь к которой формируется из параметра `path`, имени PV и PVC.

Пример пути:

```shell
/opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0
```

## Ограничения

- Невозможно задать ограничение на размер создаваемого тома.

## Примеры ресурсов LocalPathProvisioner

### ReclaimPolicy: Retain (по умолчанию)

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

### ReclaimPolicy: Delete

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

## Настройка Prometheus с использованием локального хранилища

1. Примените ресурс [LocalPathProvisioner](/modules/local-path-provisioner/cr.html#localpathprovisioner):

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

1. Убедитесь, что `spec.nodeGroups` соответствует NodeGroup, на котором будет работать Prometheus.

1. Укажите имя созданного StorageClass в настройках Prometheus:

   ```yaml
   longtermStorageClass: localpath-system
   storageClass: localpath-system
   ```

1. Дождитесь перезапуска подов Prometheus.
