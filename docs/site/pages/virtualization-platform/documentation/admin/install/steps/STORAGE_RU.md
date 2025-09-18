---
title: "Настройка хранилища"
permalink: ru/virtualization-platform/documentation/admin/install/steps/storage.html
lang: ru
---

После добавления worker-узлов необходимо настроить хранилище, которое будет использоваться для создания дисков виртуальных машин и для хранения метрик компонентов кластера. Хранилище можно выбрать из [списка поддерживаемых](/products/virtualization-platform/documentation/about/requirements.html#поддерживаемые-хранилища).

Далее рассматривается использование программно-определяемого реплицируемого блочного хранилища на базе DRBD, которое позволяет создавать реплицируемые тома на основе дискового пространства узлов. Для примера настроим StorageClass на основе томов с двумя репликами, которые располагаются на дисках `/dev/sda`.

{% alert level="info" %}
Для выполнения приведенных ниже команд необходима установленная утилита [d8](/products/virtualization-platform/reference/console-utilities/d8.html) (Deckhouse CLI) и настроеный контекст kubectl для доступа к кластеру. Также, можно подключиться к master-узлу по SSH и выполнить команду от пользователя `root` с помощью `sudo -i`.
{% endalert %}

## Включение возможности использования реплицируемого хранилища

Включите модули `sds-replicated-volume`, `snapshot-controller` и `sds-node-configurator` используя веб-интерфейс администратора, или выполните следующую команду:

```shell
d8 s module enable sds-replicated-volume
d8 s module enable snapshot-controller
d8 s module enable sds-node-configurator
```

Дождитесь включения модуля `sds-replicated-volume`:

```shell
d8 k wait module sds-replicated-volume --for='jsonpath={.status.status}=Ready' --timeout=1200s
```

Убедитесь, что все поды модуля `sds-replicated-volume` находятся в состоянии `Running` (может потребоваться некоторое время):

```shell
d8 k -n d8-sds-replicated-volume get pod -owide -w
```

## Настройка реплицируемого хранилища

Настройка хранилища включает в себя объединение доступных блочных устройств на узлах в пулы, из которых затем будет создан StorageClass.

1. Получите доступные блочные устройства:

   ```shell
   d8 k get blockdevices.storage.deckhouse.io
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

   Для создания ресурса LVMVolumeGroup, для каждого узла выполните следующие команды (укажите имя узла и имя блочного устройства):

   ```shell
   export NODE_NAME="<NODE_NAME>"
   export DEV_NAME="<BLOCK_DEVICE_NAME>"
   d8 k apply -f - <<EOF
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

   Дождитесь, что все созданные ресурсы LVMVolumeGroup перейдут в состояние `Ready`:

   ```shell
   d8 k get lvg -w
   ```

   Пример вывода:

   ```console
   NAME                THINPOOLS  CONFIGURATION APPLIED   PHASE   NODE          SIZE       ALLOCATED SIZE VG   AGE
   vg-on-master-0      0/0        True                    Ready   master-0      360484Mi   30064Mi        vg-1 29s
   vg-on-dvp-worker-1  0/0        True                    Ready   dvp-worker-1  360484Mi   30064Mi        vg-1 58s
   vg-on-dvp-worker-2  0/0        True                    Ready   dvp-worker-2  360484Mi   30064Mi        vg-1 6s
   ```

1. Создайте пул из групп томов LVM.

   Созданные группы томов нужно собрать в пул для репликации (задаётся в ресурсе ReplicatedStoragePool). Для этого выполните следующую команду (укажите имена созданных групп томов):

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: ReplicatedStoragePool
   metadata:
     name: sds-pool
   spec:
     type: LVM
     lvmVolumeGroups:
       # Укажите свои имена групп томов.
       - name: vg-on-dvp-worker-01
       - name: vg-on-dvp-worker-02
       - name: vg-on-master
   EOF
   ```

   Дождитесь, когда ресурс перейдет в состояние `Completed`:

   ```shell
   d8 k get rsp data -w
   ```

   Пример вывода:

   ```console
   NAME         PHASE       TYPE   AGE
   sds-pool     Completed   LVM    32s
   ```

1. Задайте параметры StorageClass.

   Модуль `sds-replicated-volume` использует ресурсы ReplicatedStorageClass для автоматического создания StorageClass с нужными характеристиками. В этом ресурсе важны следующие параметры:

   - `replication` — параметры репликации, для 2 реплик будет использоваться значение `Availability`;
   - `storagePool` — имя созданного ранее пула, в данном примере указывается `sds-pool`.

   Остальные параметры описаны [в документации ресурса ReplicatedStorageClass](/products/virtualization-platform/reference/cr/replicatedstorageclass.html).

   ```shell
   d8 k apply -f - <<EOF
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
   d8 k get sc
   ```

   Пример вывода:

   ```console
   NAME     PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
   sds-r2   replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   6s
   ```

1. Установите StorageClass по умолчанию (укажите имя своего объекта StorageClass):

   ```shell
   DEFAULT_STORAGE_CLASS=<DEFAULT_STORAGE_CLASS_NAME>
   d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
   ```
