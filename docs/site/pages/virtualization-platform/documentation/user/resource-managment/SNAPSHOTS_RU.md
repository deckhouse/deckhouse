---
title: "Снимки"
permalink: ru/virtualization-platform/documentation/user/resource-management/snapshots.html
lang: ru
---

Снимки предназначены для сохранения состояния ресурса в конкретный момент времени. Поддерживаются снимки дисков и снимки виртуальных машин.

## Создание снимков из дисков

Для создания снимков дисков используется ресурс [VirtualDiskSnapshot](../../../reference/cr/virtualdisksnapshot.html). Эти снимки могут служить источником данных при создании новых дисков, например, для клонирования или восстановления информации.

Чтобы гарантировать целостность данных, снимок диска можно создать в следующих случаях:

- Диск не подключен ни к одной виртуальной машине.
- ВМ выключена.
- ВМ запущена, но yстановлен qemu-guest-agent в гостевой ОС.
Файловая система успешно «заморожена» (операция fsfreeze).

Если консистентность данных не требуется (например, для тестовых сценариев), снимок можно создать:

- На работающей ВМ без "заморозки" файловой системы.
- Даже если диск подключен к активной ВМ.

Для этого в манифесте VirtualDiskSnapshot укажите:

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

Диагностика проблем с ресурсом осуществляется путем анализа информации в блоке `.status.conditions`.

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
    storageClassName: i-sds-replicated-thin-r2
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

Снимки можно использовать для реализации следующих сценариев:

- [Создание снимков из дисков](#создание-снимков-из-дисков)
- [Восстановление дисков из снимков](#восстановление-дисков-из-снимков)
- [Создание снимков виртуальных машин](#создание-снимков-виртуальных-машин)
- [Восстановление из снимков](#восстановление-из-снимков)
  - [Восстановление виртуальной машины](#восстановление-виртуальной-машины)
  - [Создание клона ВМ / Использование снимка как шаблона для создания ВМ](#создание-клона-вм--использование-снимка-как-шаблона-для-создания-вм)

![Создание снимков виртуальных машин](/../../../../images/virtualization-platform/vm-restore-clone.ru.png)

Если снимок планируется использовать как шаблон, перед его созданием выполните в гостевой ОС:

- Удаление персональных данных (файлы, пароли, история команд).
- Установку критических обновлений ОС.
- Очистку системных журналов.
- Сброс сетевых настроек.
- Удаление уникальных идентификаторов (например, через `sysprep` для Windows).
- Оптимизацию дискового пространства.
- Сброс конфигураций инициализации (`cloud-init clean`).

{% alert level="info" %}
Снимок содержит конфигурацию виртуальной машины и снимки всех её дисков.

Восстановление снимка предполагает полное восстановление виртуальной машины на момент создания её снимка.
{% endalert %}

Снимок будет создан успешно, если:

- ВМ выключена
- Установлен `qemu-guest-agent` и файловая система успешно «заморожена».

Если целостность данных не критична, снимок можно создать на работающей ВМ без заморозки ФС. Для этого укажите в спецификации:

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

При создании снимка динамический IP-адрес ВМ автоматически преобразуется в статический и сохраняется для восстановления.

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
    - storageClassName: i-sds-replicated-thin-r2 # Подставьте ваше название StorageClass.
      volumeSnapshotClassName: sds-replicated-volume # Подставьте ваше название VolumeSnapshotClass.
  requiredConsistency: true
  keepIPAddress: Never
EOF
```

## Восстановление из снимков

Для восстановления виртуальной машины из снимка используется ресурс `VirtualMachineRestore` . В процессе восстановления в кластере автоматически создаются следующие объекты:

- VirtualMachine — основной ресурс ВМ с конфигурацией из снимка.
- VirtualDisk — диски, подключенные к ВМ на момент создания снимка.
- VirtualBlockDeviceAttachment — связи дисков с ВМ (если они существовали в исходной конфигурации).
- Secret — секреты с настройками cloud-init или sysprep (если они были задействованы в оригинальной ВМ).

Важно: ресурсы создаются только в том случае , если они присутствовали в конфигурации ВМ на момент создания снимка. Это гарантирует восстановление точной копии среды, включая все зависимости и настройки.

### Восстановление виртуальной машины

{% alert level="warning" %}
Чтобы восстановить виртуальную машину, необходимо удалить её текущую конфигурацию и все связанные диски. Это связано с тем, что процесс восстановления возвращает виртуальную машину и её диски к состоянию, зафиксированному в момент создания резервного снимка.
{% endalert %}

Пример манифеста для восстановления виртуальной машины из снимка:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: <restore name>
spec:
  virtualMachineSnapshotName: <virtual machine snapshot name>
EOF
```

### Создание клона ВМ / Использование снимка как шаблона для создания ВМ

Снимок виртуальной машины может использоваться как для создания её точной копии (клона), так и в качестве шаблона для развёртывания новых ВМ с аналогичной конфигурацией.

Для этого требуется создать ресурс `VirtualMachineRestore` и задать параметры переименования в блоке `.spec.nameReplacements`, чтобы избежать конфликтов имён.

Пример манифеста для восстановления ВМ из снимка:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: <name>
spec:
  virtualMachineSnapshotName: <virtual machine snapshot name>
  nameReplacements:
    - from:
        kind: VirtualMachine
        name: <old vm name>
      to: <new vm name>
    - from:
        kind: VirtualDisk
        name: <old disk name>
      to: <new disk name>
    - from:
        kind: VirtualDisk
        name: <old secondary disk name>
      to: <new secondary disk name>
    - from:
        kind: VirtualMachineBlockDeviceAttachment
        name: <old attachment name>
      to: <new attachment name>
EOF
```

При восстановлении виртуальной машины из снимка важно учитывать следующие условия:

1. Если ресурс `VirtualMachineIPAddress` уже существует в кластере, он не должен быть назначен другой ВМ.
2. Для статических IP-адресов (`type: Static`) значение должно полностью совпадать с тем, что было зафиксировано в снимке.
3. Секреты, связанные с автоматизацией (например, конфигурация cloud-init или sysprep), должны точно соответствовать восстанавливаемой конфигурации.

Несоблюдение этих требований приведёт к ошибке восстановления . Это связано с тем, что система проверяет целостность конфигурации и уникальность ресурсов для предотвращения конфликтов в кластере.
