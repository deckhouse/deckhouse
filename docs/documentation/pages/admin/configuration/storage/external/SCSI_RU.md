---
title: "Хранилище данных на основе протокола SCSI"
permalink: ru/admin/configuration/storage/external/scsi.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) поддерживает управление хранилищами, подключёнными через iSCSI или Fibre Channel, обеспечивая возможность работы с томами на уровне блочных устройств. Это позволяет интегрировать системы хранения данных с Kubernetes и управлять ими через CSI-драйвер.

На этой странице представлены инструкции по подключению SCSI-устройств в DKP, созданию SCSITarget, StorageClass, а также проверке работоспособности системы.

## Поддерживаемые возможности

DKP поддерживает:

- обнаружение логических томов хранения данных (Logical Unit Number, LUN) через iSCSI или Fibre Channel;
- создание PersistentVolume (PV) из заранее подготовленных LUN;
- удаление PV и обнуление данных на LUN;
- подключение LUN к узлам через iSCSI или Fibre Channel;
- создание multipath-устройств и их монтирование в поды;
- отключение LUN от узлов.

## Ограничения

DKP не поддерживает:

- создание LUN на СХД;
- изменение размера LUN;
- создание снимков (snapshots).

## Системные требования

Требования к инфраструктуре и узлам кластера:

- Развёрнутая и настроенная СХД, предоставляющая доступ к LUN по iSCSI или Fibre Channel.

- Для подключения по iSCSI:
  - на каждом узле кластера должен быть настроен уникальный идентификатор iSCSI-инициатора (iSCSI Qualified Name, IQN) в файле `/etc/iscsi/initiatorname.iscsi`;
  - на узлах должен быть установлен пакет `multipath-tools`.

- Для подключения по Fibre Channel:
  - на узлах кластера должны быть установлены и доступны адаптеры Fibre Channel (Fibre Channel Host Bus Adapter, FC HBA) (`/sys/class/fc_host/host*`);
  - в сети хранения данных (Storage Area Network, SAN) должны быть настроены зонирование и маскирование LUN, обеспечивающие доступ инициаторов узлов к портам СХД;
  - необходимые LUN должны быть заранее созданы в СХД. DKP не создаёт LUN;
  - на узлах должен быть установлен пакет `multipath-tools`.

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля csi-scsi-generic

Включите [модуль `csi-scsi-generic`](/modules/csi-scsi-generic/) с помощью команды ниже. Это приведет к тому, что на всех узлах кластера будет:

- зарегистрирован CSI-драйвер;
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

Ресурс [SCSITarget](/modules/csi-scsi-generic/cr.html#scsitarget) описывает подключение к одной SCSI-цели. При создании ресурса в`spec` укажите один из способов подключения: [`iSCSI`](/modules/csi-scsi-generic/cr.html#scsitarget-v1alpha1-spec-iscsi) или [`fibreChannel`](/modules/csi-scsi-generic/cr.html#scsitarget-v1alpha1-spec-fibrechannel).

#### iSCSI

Далее приведён пример конфигурации для подключения по iSCSI.
В этом примере создаются два ресурса [SCSITarget](/modules/csi-scsi-generic/cr.html#scsitarget).

Можно создать несколько ресурсов как для одной, так и для разных СХД, что позволяет использовать multipath для повышения отказоустойчивости и производительности.

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

Проверить создание объекта можно следующей командой:

```shell
d8 k get scsitargets.storage.deckhouse.io <имя-scsitarget>
```

Объект считается успешно созданным, если в колонке его состояния (`Phase`) в выводе указано `Created`.

#### Fibre Channel

Для настройки подключения по Fibre Channel выполните следующие шаги:

1. Перед созданием ресурса SCSITarget настройте на SAN зонирование и маскирование LUN, чтобы узлы кластера получили доступ к необходимым томам.

1. Создайте ресурс SCSITarget. В поле [`spec.fibreChannel.WWNs`](/modules/csi-scsi-generic/cr.html#scsitarget-v1alpha1-spec-fibrechannel-wwns) укажите World Wide Port Name (WWPN) портов СХД, через которые доступны необходимые LUN. DKP обнаруживает устройства, сопоставляя указанные WWPN с путями в директории `/dev/disk/by-path/`.

   WWPN следует указывать в одном из следующих форматов:

   - 16 шестнадцатеричных символов (`2001c89f1acd6117`);
   - с двоеточиями (`20:01:c8:9f:1a:cd:61:17`);
   - с префиксом `0x` (`0x2001c89f1acd6117`).

   При обнаружении и подключении тома DKP выполняет сканирование FC-хостов. При этом он не настраивает коммутаторы и не выполняет вход на целевые устройства.

   Пример конфигурации с двумя портами СХД для организации multipath:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: SCSITarget
   metadata:
     name: hpe-3par-fc-1
   spec:
     deviceTemplate:
       metadata:
         labels:
           my-key: some-label-value
     fibreChannel:
       WWNs:
       - 2001c89f1acd6117

   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: SCSITarget
   metadata:
     name: hpe-3par-fc-2
   spec:
     deviceTemplate:
       metadata:
         labels:
           my-key: some-label-value
     fibreChannel:
       WWNs:
       - 2001c89f1acd6118
   EOF
   ```

1. Перед применением ресурса убедитесь, что на узле доступны адаптеры FC HBA и что после настройки зонирования на SAN видны пути к LUN:

   ```shell
   # FC-хосты присутствуют.
   ls /sys/class/fc_host/

   # WWPN целевых устройств и LUN доступны после настройки зонирования.
   ls -l /dev/disk/by-path/ | grep -E 'fc-|/fc-'
   ```

   Результаты проверки:

   - Если директория `/sys/class/fc_host/` пуста, на узле отсутствуют FC HBA или не загружен драйвер для них.
   - Если в директории `/dev/disk/by-path/` отсутствуют записи вида `fc-*`, проверьте настройки зонирования и маскирование LUN на СХД.
   - Если FC-пути отображаются, узел готов обнаруживать LUN, предоставленные через Fibre Channel.

1. После создания SCSITarget контроллер обнаружит доступные LUN и создаст объекты SCSIDevice. В дальнейшем используйте тот же ресурс [SCSIStorageClass](/modules/csi-scsi-generic/cr.html#scsistorageclass) и селектор [`scsiDeviceSelector`](/modules/csi-scsi-generic/cr.html#scsistorageclass-v1alpha1-spec-scsideviceselector), что и для iSCSI.

1. Проверьте создание объекта следующей командой:

   ```shell
   d8 k get scsitargets.storage.deckhouse.io <имя-scsitarget>
   ```

   Объект считается успешно созданным, если в колонке его состояния (`Phase`) в выводе указано `Created`.

### Создание StorageClass

Для создания StorageClass используйте ресурс [SCSIStorageClass](/modules/csi-scsi-generic/cr.html#scsistorageclass). Пример команды для создания такого ресурса:

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

Параметр [`scsiDeviceSelector`](/modules/csi-scsi-generic/cr.html#scsistorageclass-v1alpha1-spec-scsideviceselector) позволяет выбрать SCSITarget для создания PV по лейблам. В примере выше выбираются все SCSITarget с лейблом `my-key: some-label-value`. Этот лейбл будет назначен на все устройства, которые будут обнаружены в указанных SCSITarget.

Проверить создание объекта можно следующей командой:

```shell
d8 k get scsistorageclasses.storage.deckhouse.io <имя-scsistorageclass>
```

Объект считается успешно созданным, если в колонке его состояния (`Phase`) в выводе указано `Created`.

### Проверка работоспособности модуля csi-scsi-generic

Проверьте состояние подов в неймспейсе `d8-csi-scsi-generic` при помощи команды ниже. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
d8 k -n d8-csi-scsi-generic get pod -owide -w
```
