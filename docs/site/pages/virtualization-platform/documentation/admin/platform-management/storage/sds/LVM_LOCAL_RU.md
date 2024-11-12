---
title: "Локальное на основе LVM"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html
lang: ru
---

Чтобы создать локальные блочные StorageClass’ы на базе LVM (Logical Volume Manager), можно использовать модуль sds-local-volume.  

## Включение модуля

Настройка LVM осуществляется модулем sds-node-configurator, который необходимо включить перед включением модуля sds-local-volume.
Чтобы сделать это, примените следующий ресурс `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-node-configurator
spec:
  enabled: true
  version: 1
EOF
```

Дождитесь, когда модуль sds-node-configurator перейдет в состояние Ready. 
На этом этапе НЕ нужно проверять поды в namespace d8-sds-node-configurator.

```shell
d8 k get modules sds-node-configurator -w

# NAME                    WEIGHT   STATE     SOURCE      STAGE   STATUS
# sds-node-configurator   900      Enabled   deckhouse           Ready
```

Затем, чтобы включить модуль sds-local-volume с настройками по умолчанию, выполните команду. 

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-local-volume
spec:
  enabled: true
  version: 1
EOF
```

Это приведет к тому, что на всех узлах кластера будут запущены служебные поды компонентов sds-local-volume.

```shell
d8 k get modules sds-local-volume -w

# NAME               WEIGHT   STATE     SOURCE     STAGE   STATUS
# sds-local-volume   920      Enabled   Embedded           Ready
```

Чтобы проверить, что в namespace d8-sds-local-volume и d8-sds-node-configurator все поды в состоянии Running или Completed 
и запущены на всех узлах, где планируется использовать ресурсы LVM, можно использовать команды:

```shell
d8 k -n d8-sds-local-volume get pod -w
d8 k -n d8-sds-node-configurator get pod -w
```

## Подготовка узлов

Перед тем как создавать StorageClass’ы необходимо создать на узлах группы томов LVM с помощью пользовательских ресурсов `LVMVolumeGroup`.

Первым делом получите все ресурсы `BlockDevice`, которые доступны в вашем кластере:

```shell
d8 k get bd

# NAME                                           NODE       CONSUMABLE   SIZE           PATH
# dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
# dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
# dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
# dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
# dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
# dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
```

В примере выполнения команды выше в наличии имеется 6 блочных устройств, расположенных на 3 узлах.

Перед созданием `LVMVolumeGroup` убедитесь, что на данном узле запущен pod sds-local-volume-csi-node. Сделать это можно командой:

```shell
kubectl -n d8-sds-local-volume get pod -l app=sds-local-volume-csi-node -owide

# NAME                              READY   STATUS    RESTARTS   AGE   IP             NODE       NOMINATED NODE   READINESS GATES
# sds-local-volume-csi-node-c7mdp   3/3     Running   0          1h    10.111.1.148   worker-0   <none>           <none>
# sds-local-volume-csi-node-g7kpz   3/3     Running   0          1h    10.111.2.214   worker-1   <none>           <none>
# sds-local-volume-csi-node-xkr8l   3/3     Running   0          1h    10.111.0.157   worker-2   <none>           <none>
```

## Создание группы томов LVM 

Чтобы создать `LVMVolumeGroup` для узла worker-0 примените следующий ресурс, предварительно заменив имена узла и блочных устройств на свои:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-worker-0"
spec:
  type: Local
  local:
    # Замените на имя своего узла, для которого создаете группу томов. 
    nodeName: "worker-0"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Замените на имена своих блочных устройств узла, для которого создаете группу томов. 
          - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
          - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
  # Имя группы томов LVM, которая будет создана из указанных выше блочных устройств на выбранном узле.
  actualVGNameOnTheNode: "vg"
EOF
```

Дождитесь, когда созданный ресурс `LVMVolumeGroup` перейдет в состояние `Operational`.

```shell
d8 k get lvg vg-on-worker-0 -w

# NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
# vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

Если ресурс перешел в состояние Operational, то это значит, что на узле worker-0 
из блочных устройств /dev/nvme1n1 и /dev/nvme0n1p6 была создана группа томов LVM с именем vg.

Далее необходимо повторить создание ресурсов `LVMVolumeGroup` для оставшихся узлов (worker-1 и worker-2),
изменив в примере выше имя ресурса `LVMVolumeGroup`, имя узла и имена блочных устройств, соответствующих узлу.

## Создание StorageClass’а

Убедитесь, что все созданные ресурсы `LVMVolumeGroup` перешли в состояние `Operational`.

```shell
d8 k get lvg -w

# NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
# vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
# vg-on-worker-1   1/1         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
# vg-on-worker-2   1/1         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-on-worker-0
      - name: vg-on-worker-1
      - name: vg-on-worker-2
    type: Thick
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Проверьте, что созданный ресурс `LocalStorageClass` перешел в состояние `Created` и соответствующий `StorageClass` создался:

```shell
d8 k get lsc local-storage-class -w

# NAME                  PHASE     AGE
# local-storage-class   Created   1h

d8 k get sc local-storage-class

# NAME                  PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# local-storage-class   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

Если `StorageClass` с именем local-storage-class появился, значит настройка модуля sds-local-volume завершена. 
Теперь пользователи могут создавать `PersistentVolume`, указывая `StorageClass` с именем local-storage-class.
