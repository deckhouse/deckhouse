---
title: "Настройка хранилища"
permalink: ru/virtualization-platform/documentation/admin/install/steps/storage.html
lang: ru
---

## Настройка хранилища

После добавления worker-узлов необходимо настроить хранилище, которое будет использоваться для создания дисков виртуальных машин и для хранения метрик компонентов кластера. Хранилище можно выбрать из [списка поддерживаемых](/products/virtualization-platform/documentation/about/requirements.html#поддерживаемые-хранилища).

Далее рассмотрим включение и настройку программно-определяемого хранилища `sds-replicated-volume`. Это хранилище позволяет создать реплицируемые тома на основе дискового пространства узлов. Для примера настроим StorageClass на основе томов с двумя репликами, которые располагаются на дисках `/dev/sda`.

## Добавление sds-replicated-volume

Для добавления хранилища `sds-replicated-volume` нужно включить два модуля Deckhouse, создав ресурсы ModuleConfig:

```yaml
sudo -i d8 k create -f - <<EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-node-configurator
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-replicated-volume
spec:
  enabled: true
EOF
```

Дождитесь включения модуля:

```shell
sudo -i d8 k wait module sds-replicated-volume --for='jsonpath={.status.status}=Ready' --timeout=1200s
```

Убедитесь, что все поды модуля `sds-replicated-volume` находятся в состоянии `Running` (может потребоваться некоторое время):

```shell
sudo -i d8 k -n d8-sds-replicated-volume get pod -owide -w
```

## Настройка sds-replicated-volume

Настройка хранилища включает в себя объединение доступных блочных устройств на узлах в пулы, из которых затем будет создан StorageClass.

1. Получите доступные блочные устройства:

   ```shell
   sudo -i d8 k get blockdevices.storage.deckhouse.io
   ```

   Пример вывода с дополнительными дисками sda:

   ```console
   NAME                                           NODE           CONSUMABLE   SIZE          PATH        AGE
   dev-93640bc74158c6e491a2f257b5e0177309588db0   master-0       false        468851544Ki   /dev/sda    8m28s
   dev-40bf7a561aee502f20b81cf1eff873a0455a95cb   dvp-worker-1   false        468851544Ki   /dev/sda    8m17s
   dev-b1c720a7cec32ae4361de78b71f08da1965b1d0c   dvp-worker-2   false        468851544Ki   /dev/sda    8m12s
   ```

1. Создайте VolumeGroup на каждом узле.

   На каждом узле необходимо создать группу томов LVM с помощью ресурса [LVMVolumeGroup](/products/virtualization-platform/reference/cr/lvmvolumegroup.html).

   Для создания ресурса LVMVolumeGroup на узле используйте следующие команды:

   ```yaml
   export NODE_NAME="dvp-worker-1"
   export DEV_NAME="dev-40bf7a561aee502f20b81cf1eff873a0455a95cb"
   sudo -i d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-on-${NODE_NAME}"
   spec:
     type: Local
     local:
       nodeName: "$NODE_NAME"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - "$DEV_NAME"
     # Имя группы томов LVM, которая будет создана из указанных выше блочных устройств на выбранном узле.
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

   Повторите действия для каждого узла, блочное устройство которого планируется использовать. В примере это все три узла: `master-0`, `dvp-master-1` и `dvp-master-2`.

   Дождитесь, что все созданные ресурсы LVMVolumeGroup перейдут в состояние `Ready`:

   ```shell
   sudo -i d8 k get lvg -w
   ```

   Пример вывода:

   ```console
   NAME                THINPOOLS  CONFIGURATION APPLIED   PHASE   NODE          SIZE       ALLOCATED SIZE VG   AGE
   vg-on-master-0      0/0        True                    Ready   master-0      360484Mi   30064Mi        vg-1 29s
   vg-on-dvp-worker-1  0/0        True                    Ready   dvp-worker-1  360484Mi   30064Mi        vg-1 58s
   vg-on-dvp-worker-2  0/0        True                    Ready   dvp-worker-2  360484Mi   30064Mi        vg-1 6s
   ```

1. Создайте пул из групп томов LVM.

   Созданные группы томов нужно собрать в пул для репликации. Пул задаётся в ресурсе ReplicatedStoragePool:

   ```yaml
   sudo -i d8 k apply -f - <<EOF
    apiVersion: storage.deckhouse.io/v1alpha1
    kind: ReplicatedStoragePool
    metadata:
      name: sds-pool
    spec:
      type: LVM
      lvmVolumeGroups:
        - name: vg-on-dvp-worker-01
        - name: vg-on-dvp-worker-02
        - name: vg-on-master
   EOF
   ```

   Дождитесь, когда ресурс перейдет в состояние `Completed`:

   ```shell
   sudo -i d8 k get rsp data -w
   ```

   Пример вывода:

   ```console
   NAME         PHASE       TYPE   AGE
   sds-pool     Completed   LVM    32s
   ```

1. Задайте параметры StorageClass.

   Модуль `sds-replicated-volume` использует ресурсы ReplicatedStorageClass для автоматического создания StorageClass'ов с нужными характеристиками. В этом ресурсе важны следующие параметры:

   - `replication` — параметры репликации, для 2 реплик будет использоваться значение `Availability`;
   - `storagePool` — имя созданного ранее пула, в данном примере указывается `sds-pool`.

   Остальные параметры описаны [в документации ресурса ReplicatedStorageClass](/products/virtualization-platform/reference/cr/replicatedstorageclass.html).

   ```yaml
   sudo -i d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStorageClass
   metadata:
     name: sds-r2
   spec:
     replication: Availability
     storagePool: sds-pool
     reclaimPolicy: Delete
     topology: Ignored
   EOF
   ```

   Проверьте, что в кластере появился соответствующий StorageClass:

   ```shell
   sudo -i d8 k get sc
   ```

   Пример вывода:

   ```console
   NAME     PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
   sds-r2   replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   6s
   ```

1. Установите StorageClass по умолчанию:

   ```shell
   # Укажите имя своего объекта StorageClass.
   DEFAULT_STORAGE_CLASS=replicated-storage-class
   sudo -i d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
   ```
