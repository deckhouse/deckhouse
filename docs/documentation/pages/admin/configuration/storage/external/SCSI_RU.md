---
title: "Хранилище данных на основе протокола SCSI"
permalink: ru/admin/storage/external/scsi.html
lang: ru
---

Deckhouse поддерживает управление хранилищами, подключёнными через iSCSI или Fibre Channel, обеспечивая возможность работы с томами на уровне блоковых устройств. Это позволяет интегрировать системы хранения данных с Kubernetes и управлять ими через CSI-драйвер.

На этой странице представлены инструкции по подключению SCSI-устройств в Deckhouse, созданию SCSITarget, StorageClass, а также проверке работоспособности системы.

### Поддерживаемые возможности

- Обнаружение LUN через iSCSI/FC.
- Создание PersistentVolume из заранее подготовленных LUN.
- Удаление PersistentVolume и очистка данных на LUN.
- Подключение LUN к узлам через iSCSI/FC.
- Создание `multipath`-устройств и их монтирование в поды.
- Отключение LUN от узлов.

### Ограничения

- Невозможно создать LUN на СХД.
- Нельзя изменить размер LUN.
- Снимки (snapshots) не поддерживаются.

## Системные требования

- Настроенная и доступная СХД с подключением через iSCSI/FC.
- Уникальные IQN на каждом узле Kubernetes в файле `/etc/iscsi/initiatorname.iscsi`.

## Настройка и конфигурация

Все команды выполняются на машине с административным доступом к API Kubernetes.

### Включение модуля

Для работы с хранилищами, подключёнными через SCSI, включите модуль `csi-scsi-generic`. Это приведет к:
- Регистрации CSI-драйвера.
- Запуску сервисных подов `csi-scsi-generic`.

```yaml
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

Дождитесь, пока модуль перейдет в состояние `Ready`. Проверьте состояние модуля командой:

```shell
d8 k get module csi-scsi-generic -w
```

### Создание SCSITarget

Для работы с SCSI-устройствами создайте ресурсы [SCSITarget](../../../reference/cr/scsitarget/).

```yaml
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

Пример команд для FC ресурса:

```yaml
d8 k apply -f -<<EOF
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

Обратите внимание, что в примере выше используются два [SCSITarget](../../../reference/cr/scsitarget/). Таким образом можно создать несколько [SCSITarget](../../../reference/cr/scsitarget/) как для одного, так и для разных СХД. Это позволяет использовать `multipath` для повышения отказоустойчивости и производительности.

Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

```shell
d8 k get scsitargets.storage.deckhouse.io <имя scsitarget>
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурс [SCSIStorageClass](../../../reference/cr/scsistorageclass/). Пример команд для создания такого ресурса:

```yaml
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

Обратите внимание на `scsiDeviceSelector`. Этот параметр позволяет выбрать [SCSITarget](../../../reference/cr/scsitarget/) для создания PersistentVolume по меткам. В примере выше выбираются все [SCSITarget](../../../reference/cr/scsitarget/) с меткой `my-key: some-label-value`. Эта метка будет выставлена на все устройства, которые будут обнаружены в указанных [SCSITarget](../../../reference/cr/scsitarget/).

Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

```shell
d8 k get scsistorageclasses.storage.deckhouse.io <имя scsistorageclass>
```

### Проверка работоспособности модуля

Проверьте состояние подов в пространстве `d8-csi-scsi-generic` при помощи следующей команды. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
d8 k -n d8-csi-scsi-generic get pod -owide -w
```
