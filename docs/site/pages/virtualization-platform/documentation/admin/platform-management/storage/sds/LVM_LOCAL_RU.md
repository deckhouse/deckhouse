---
title: "Локальное на основе LVM"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html
lang: ru
---

Использование локального хранилища устраняет сетевую задержку, что приводит к более высокой производительности чтения
и записи данных по сравнению с удаленными хранилищами, которые требуют сетевого доступа.
Поскольку данные не проходят через сеть, уменьшаются нагрузка и расходы на сетевую инфраструктуру.

Чтобы создать локальные блочные StorageClass’ы, можно использовать модуль sds-local-volume.  

## Включение модуля

Настройка локального блочного хранилища происходит на основе логического менеджера томов LVM (Logical Volume Manager).
Управление LVM осуществляется модулем sds-node-configurator, который необходимо включить перед включением модуля sds-local-volume.
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

## Преднастройка узлов

Убедитесь, что на всех узлах, где планируется использовать ресурсы LVM, запущены служебные поды sds-local-volume-csi-node, 
которые обеспечивают взаимодействие с узлами, на которых расположены компоненты LVM. 
Сделать это можно с помощью следующей команды:

```shell
kubectl -n d8-sds-local-volume get pod -l app=sds-local-volume-csi-node -owide

# NAME                              READY   STATUS    RESTARTS   AGE   IP             NODE       NOMINATED NODE   READINESS GATES
# sds-local-volume-csi-node-c7mdp   3/3     Running   0          1h    10.111.1.148   worker-0   <none>           <none>
# sds-local-volume-csi-node-g7kpz   3/3     Running   0          1h    10.111.2.214   worker-1   <none>           <none>
# sds-local-volume-csi-node-xkr8l   3/3     Running   0          1h    10.111.0.157   worker-2   <none>           <none>
```

Размещение данных pod’ов по узлам определяется на основе специальных меток (nodeSelector), которые указываются в поле
spec.settings.dataNodes.nodeSelector в настройках модуля. Для получения более подробной информации о настройке, пожалуйста,
перейдите по [ссылке](todo,mc).

Процесс дальнейшей преднастройки описан по [ссылке](todo,mc).

## Создание StorageClass’а

Убедитесь, что все созданные ресурсы `LVMVolumeGroup` перешли в состояние `Operational`.

```shell
d8 k get lvg -w

# NAME             THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
# vg-on-worker-0   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
# vg-on-worker-1   1/1         True                    Ready   worker-1   360484Mi   30064Mi          vg   1h
# vg-on-worker-2   1/1         True                    Ready   worker-2   360484Mi   30064Mi          vg   1h
```

Создание StorageClass’ов осуществляется через ресурс `LocalStorageClass`, который определяет конфигурацию для желаемого
класса хранения. Ручное создание ресурса `StorageClass` без `LocalStorageClass` может привести к нежелательному поведению.
Пример создания ресурса `LocalStorageClass`, `PersistentVolumes` которого будут размещены на группах томов на трех узлах:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class-thick
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

Созданный выше LocalStorageClass имеет тип Thick, который обладает высокой производительностью, сравнимой 
с производительностью накопителя, но не позволяет использовать snapshot’ы.

Альтернативно можно создать с типом Thin, что позволит использовать snapshot’ы и overprovisioning, но производительность 
будет ниже. Пример такого ресурса:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LocalStorageClass
metadata:
  name: local-storage-class-thin
spec:
  lvm:
    lvmVolumeGroups:
      - name: vg-on-worker-1
        thin:
          - name: thin-pool
      - name: vg-on-worker-2
        thin:
        - name: thin-pool
    type: Thin
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

Проверьте, что созданный ресурс `LocalStorageClass` перешел в состояние `Created` и соответствующий `StorageClass` создался:

```shell
d8 k get lsc local-storage-class -w

# NAME                        PHASE     AGE
# local-storage-class-thick   Created   1h
# local-storage-class-thin    Created   1h

d8 k get sc local-storage-class

# NAME                        PROVISIONER                      RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# local-storage-class-thick   local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
# local-storage-class-thin    local.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   1h
```

Если StorageClass'ы появились, значит настройка модуля sds-local-volume завершена. 
Теперь пользователи могут создавать `PersistentVolume`, указывая созданные StorageClass`ы.
