---
title: "SCSI-хранилище"
permalink: ru/storage/admin/external/scsi.html
lang: ru
---

Данный модуль хранилища предоставляет CSI для управления томами c использованием СХД с подключением через iSCSI/FC.

На данный момент поддерживается:
- обнаружение LUN через iSCSI/FC
- создание PV из заранее подготовленных LUN
- удаление PV и обнуление данных на LUN
- подключение LUN к узлам через iSCSI/FC
- создание multipath устройств и монтирование их в поды
- отключение LUN от узлов

Не поддерживается:
- создание LUN на СХД
- изменение размера LUN
- создание снимков

## Системные требования и рекомендации

### Требования

- Наличие развернутой и настроенной СХД с подключением через iSCSI/FC.
- Уникальные iqn в /etc/iscsi/initiatorname.iscsi на каждой из Kubernetes Nodes

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

- Включить модуль `csi-scsi-generic`.  Это приведет к тому, что на всех узлах кластера будет:
  - зарегистрирован CSI драйвер;
  - запущены служебные поды компонентов `csi-scsi-generic`.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-scsi-generic
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
kubectl get module csi-scsi-generic -w
```

### Создание SCSITarget

Для создания SCSITarget необходимо использовать ресурс `SCSITarget`. Пример команд для создания такого ресурса:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSITarget
metadata:
  name: hpe-3par-1
spec:
  deviceTemplate:
    metadata:
      labels:
        my-key: some-label-value
  iSCSI:
    auth:
      login: ""
      password: ""
    iqn: iqn.2000-05.com.3pardata:xxxx1
    portals:
    - 192.168.1.1

---
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSITarget
metadata:
  name: hpe-3par-2
spec:
  deviceTemplate:
    metadata:
      labels:
        my-key: some-label-value
  iSCSI:
    auth:
      login: ""
      password: ""
    iqn: iqn.2000-05.com.3pardata:xxxx2
    portals:
    - 192.168.1.2
EOF

```


Пример команд для FC ресурса:

```shell
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSITarget
metadata:
  name: scsi-target-2
spec:
  fibreChannel:
    WWNs:
      - 00:00:00:00:00:00:00:00
      - 00:00:00:00:00:00:00:01
  deviceTemplate:
    metadata:
      labels:
        some-label-key: some-label-value1
EOF

```





Обратите внимание, что в примере выше используются два SCSITarget. Таким образом можно создать несколько SCSITarget как для одного, так и для разных СХД. Это позволяет использовать multipath для повышения отказоустойчивости и производительности.

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get scsitargets.storage.deckhouse.io <имя scsitarget>
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурс `SCSIStorageClass`. Пример команд для создания такого ресурса:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSIStorageClass
metadata:
  name: scsi-all
spec:
  scsiDeviceSelector:
    matchLabels:
      my-key: some-label-value
  reclaimPolicy: Delete
EOF
```

Обратите внимание на `scsiDeviceSelector`. Этот параметр позволяет выбрать SCSITarget для создания PV по меткам. В примере выше выбираются все SCSITarget с меткой `my-key: some-label-value`. Эта метка будет выставлена на все девайсы, которые будут обнаружены в указанных SCSITarget.
- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get scsistorageclasses.storage.deckhouse.io <имя scsistorageclass>
```

### Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-scsi-generic`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
kubectl -n d8-csi-scsi-generic get pod -owide -w
```

