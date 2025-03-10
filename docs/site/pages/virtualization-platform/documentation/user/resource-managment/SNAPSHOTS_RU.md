---
title: "Снимки"
permalink: ru/virtualization-platform/documentation/user/resource-management/snapshots.html
lang: ru
---

Снимки предназначены для сохранения состояния ресурса в конкретный момент времени. Поддерживаются снимки дисков и снимки виртуальных машин.

## Создание снимков из дисков

Для создания снимков дисков используется ресурс [VirtualDiskSnapshot](../../../reference/cr/virtualdisksnapshot.html). Он может быть использован в качестве источников данных для создания новых виртуальных дисков.

Для гарантии целостности и консистентности данных, снимок диска можно создать в следующих случаях:

- виртуальный диск не подключен ни к одной виртуальной машине;
- виртуальный диск подключен к виртуальной машине, которая выключена;
- виртуальный диск подключен к запущенной виртуальной машине, в ОС виртуальной машины установлен агент (`qemu-guest-agent`), операция по «заморозке» файловой системы прошла успешно.

Если целостность и консистентность неважна, снимок можно выполнить на работающей виртуальной машине и без «заморозки» файловой системы. Для этого в спецификации ресурса `VirtualDiskSnapshot` добавьте:

```yaml
spec:
  requiredConsistency: false
```

При создании снимка требуется указать название класса снимка томов `VolumeSnapshotClass`, который будет использоваться для создания снимка.

Для получения списка поддерживаемых ресурсов `VolumeSnapshotClass` выполните команду:

```shell
d8 k get volumesnapshotclasses
```

Пример вывода:

```console
NAME                     DRIVER                                DELETIONPOLICY   AGE
csi-nfs-snapshot-class   nfs.csi.k8s.io                        Delete           34d
sds-replicated-volume    replicated.csi.storage.deckhouse.io   Delete           39d
```

Пример манифеста для создания снимка диска:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDiskSnapshot
metadata:
  name: linux-vm-root-snapshot
spec:
  requiredConsistency: true
  virtualDiskName: linux-vm-root
  volumeSnapshotClassName: sds-replicated-volume
EOF
```

Для просмотра списка снимков дисков, выполните команду:

```shell
d8 k get vdsnapshot
```

Пример вывода:

```console
NAME                   PHASE     CONSISTENT   AGE
linux-vm-root-snapshot Ready     true         3m2s
```

После создания, `VirtualDiskSnapshot` может находиться в следующих состояниях:

- `Pending` - ожидание готовности всех зависимых ресурсов, требующихся для создания снимка.
- `InProgress` — идет процесс создания снимка виртуального диска.
- `Ready` — создание снимка успешно завершено, и снимок виртуального диска доступен для использования.
- `Failed` — произошла ошибка во время процесса создания снимка виртуального диска.
- `Terminating` — ресурс находится в процессе удаления.

## Восстановление дисков из снимков

Для того чтобы восстановить диск из ранее созданного снимка диска, необходимо в качестве `dataSource` указать соответствующий объект:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Настройки параметров хранения диска.
  persistentVolumeClaim:
    # Укажите размер больше чем значение.
    size: 10Gi
    # Подставьте ваше название StorageClass.
    storageClassName: i-linstor-thin-r2
  # Источник, из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-snapshot
EOF
```

## Создание снимков виртуальных машин

Для создания снимков виртуальных машин используется ресурс [VirtualMachineSnapshot](../../../reference/cr/virtualmachinesnapshot.html).

Чтобы гарантировать целостность и консистентность данных, снимок виртуальной машины будет создан, если выполняется хотя бы одно из следующих условий:

- виртуальная машина выключена;
- в операционной системе виртуальной машины установлен агент `qemu-guest-agent`, и операция по «заморозке» файловой системы прошла успешно.

Если целостность и консистентность неважны, снимок можно создать на работающей виртуальной машине и без «заморозки» файловой системы. Для этого в спецификации ресурса [VirtualMachineSnapshot](../../../reference/cr/virtualmachinesnapshot.html) укажите:

```yaml
spec:
  requiredConsistency: false
```

При создании снимка необходимо указать названия классов снимков томов `VolumeSnapshotClass`, которые будут использованы для создания снимков дисков, подключенных к виртуальной машине.

Чтобы получить список поддерживаемых ресурсов `VolumeSnapshotClass`, выполните команду:

```shell
d8 k get volumesnapshotclasses
```

Пример вывода:

```console
NAME                     DRIVER                                DELETIONPOLICY   AGE
csi-nfs-snapshot-class   nfs.csi.k8s.io                        Delete           34d
sds-replicated-volume    replicated.csi.storage.deckhouse.io   Delete           39d
```

Снимок виртуальной машины не будет создан, если выполнится хотя бы одно из следующих условий:

- не все зависимые устройства виртуальной машины готовы;
- есть изменения, ожидающие перезапуска виртуальной машины;
- среди зависимых устройств есть диск, находящийся в процессе изменения размера.

При создании снимка виртуальной машины IP-адрес будет преобразован в статичный и будет использован позже при восстановлении виртуальной машины из снимка.

Если не требуется преобразование и использование старого IP-адреса виртуальной машины, можно установить соответствующую политику в значение `Never`. В этом случае будет использован тип адреса без преобразования (`Auto` или `Static`).

```yaml
spec:
  keepIPAddress: Never
```

Пример манифеста для создания снимка виртуальной машины:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshot
metadata:
  name: linux-vm-snapshot
spec:
  virtualMachineName: linux-vm
  volumeSnapshotClasses:
    - storageClassName: i-linstor-thin-r2 # Подставьте ваше название StorageClass.
      volumeSnapshotClassName: sds-replicated-volume # Подставьте ваше название VolumeSnapshotClass.
  requiredConsistency: true
  keepIPAddress: Never
EOF
```

## Восстановление виртуальных машин из снимков

Для восстановления виртуальных машин из снимков используется ресурс [VirtualMachineRestore](../../../reference/cr/virtualmachinerestore.html).

В процессе восстановления будет создана новая виртуальная машина, а также все её зависимые ресурсы (диски, IP-адрес, ресурс со сценарием автоматизации (Secret) и ресурсы для динамического подключения дисков [VirtualMachineBlockDeviceAttachment](../../../reference/cr/virtualmachineblockdeviceattachment.html)) .

Если возникает конфликт имен между существующими и восстанавливаемыми ресурсами для [VirtualMachine](../../../reference/cr/virtualmachine.html), [VirtualDisk](../../../reference/cr/virtualdisk.html) или [VirtualMachineBlockDeviceAttachment](../../../reference/cr/virtualmachineblockdeviceattachment.html), восстановление не будет успешно. Чтобы избежать этого, используйте параметр `nameReplacements`.

Если восстанавливаемый ресурс [VirtualMachineIPAddress](../../../reference/cr/virtualmachineipaddress.html) уже присутствует в кластере, он не должен быть присоединен к другой виртуальной машине, а если это ресурс типа `Static`, его IP-адрес должен совпадать. Восстанавливаемый секрет с автоматизацией также должен полностью соответствовать восстановленному. Несоблюдение этих условий приведет к неудаче восстановления.

Пример манифеста для восстановления виртуальной машины из снимка:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: linux-vm-restore
spec:
  virtualMachineSnapshotName: linux-vm-snapshot
  nameReplacements:
    - from:
        kind: VirtualMachine
        name: linux-vm
      to: linux-vm-2 # Воссоздание существующей виртуальной машины linux-vm с новым именем linux-vm-2.
    - from:
        kind: VirtualDisk
        name: linux-vm-root
      to: linux-vm-root-2 # Воссоздание существующего виртуального диска linux-vm-root с новым именем linux-vm-root-2.
    - from:
        kind: VirtualDisk
        name: blank-disk
      to: blank-disk-2 # Воссоздание существующего виртуального диска blank-disk с новым именем blank-disk-2.
    - from:
        kind: VirtualMachineBlockDeviceAttachment
        name: attach-blank-disk
      to: attach-blank-disk-2 # Воссоздание существующего виртуального диска attach-blank-disk с новым именем attach-blank-disk-2.
EOF
```
