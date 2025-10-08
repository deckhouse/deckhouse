---
title: "Хранилище данных на основе протокола SCSI"
permalink: ru/admin/configuration/storage/external/scsi.html
lang: ru
---

Deckhouse поддерживает управление хранилищами, подключёнными через iSCSI или Fibre Channel, обеспечивая возможность работы с томами на уровне блоковых устройств. Это позволяет интегрировать системы хранения данных с Kubernetes и управлять ими через CSI-драйвер.

На этой странице представлены инструкции по подключению SCSI-устройств в Deckhouse, созданию SCSITarget, StorageClass, а также проверке работоспособности системы.

## Поддерживаемые возможности

- Обнаружение LUN через iSCSI/FC.
- Создание PersistentVolume из заранее подготовленных LUN.
- Удаление PersistentVolume и очистка данных на LUN.
- Подключение LUN к узлам через iSCSI/FC.
- Создание `multipath`-устройств и их монтирование в поды.
- Отключение LUN от узлов.

## Ограничения

- Невозможно создать LUN на СХД.
- Нельзя изменить размер LUN.
- Снимки (snapshots) не поддерживаются.

## Системные требования

- Наличие развернутой и настроенной СХД с подключением через SCSI.
- Уникальные IQN в `/etc/iscsi/initiatorname.iscsi` на каждом узле Kubernetes.

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

Включите [модуль `csi-scsi-generic`](/modules/csi-scsi-generic/). Это приведет к тому, что на всех узлах кластера будет:

- зарегистрирован CSI драйвер;
- запущены служебные поды компонентов `csi-scsi-generic`.

```shell
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-scsi-generic
spec:
  enabled: true
  version: 1
EOF
```

Дождитесь, когда модуль перейдет в состояние `Ready`:

```shell
d8 k get module csi-scsi-generic -w
```

### Создание SCSITarget

Для создания SCSITarget необходимо использовать ресурс [SCSITarget](/modules/csi-scsi-generic/cr.html#scsitarget). Пример команд для создания такого ресурса:

```shell
d8 k apply -f -<<EOF
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

Обратите внимание, что в примере выше используются два SCSITarget. Таким образом можно создать несколько SCSITarget как для одного, так и для разных СХД. Это позволяет использовать multipath для повышения отказоустойчивости и производительности.

Проверить создание объекта можно командой (`Phase` должен быть `Created`):

```shell
d8 k get scsitargets.storage.deckhouse.io <имя scsitarget>
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурс [SCSIStorageClass](/modules/csi-scsi-generic/cr.html#scsistorageclass). Пример команды для создания такого ресурса:

```shell
d8 k apply -f -<<EOF
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

Обратите внимание на `scsiDeviceSelector`. Этот параметр позволяет выбрать SCSITarget для создания PV по лейблам. В примере выше выбираются все SCSITarget с лейблом `my-key: some-label-value`. Этот лейбл будет назначен на все девайсы, которые будут обнаружены в указанных SCSITarget.

Проверить создание объекта можно командой (`Phase` должен быть `Created`):

```shell
d8 k get scsistorageclasses.storage.deckhouse.io <имя scsistorageclass>
```

### Проверка работоспособности модуля

Проверьте состояние подов в пространстве `d8-csi-scsi-generic` при помощи следующей команды. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
d8 k -n d8-csi-scsi-generic get pod -owide -w
```
