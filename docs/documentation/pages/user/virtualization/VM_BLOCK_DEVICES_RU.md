---
title: "Подключение дисков и образов к виртуальной машине"
permalink: ru/user/virtualization/vm-block-devices.html
description: "Подключение дисков и образов к виртуальной машине через спецификацию и ресурс VirtualMachineBlockDeviceAttachment, порядок загрузки и именование устройств."
search: подключение диска, VirtualMachineBlockDeviceAttachment, VMBDA, CD-ROM, порядок загрузки
lang: ru
---

К виртуальной машине можно подключать диски и образы. Они описываются как блочные устройства (BlockDevices).

Типы блочных устройств и режимы доступа:

| Тип блочного устройства                                                    | Комментарий                                                       |
|----------------------------------------------------------------------------|-------------------------------------------------------------------|
| [VirtualImage](/modules/virtualization/cr.html#virtualimage)               | Подключается в режиме для чтения, или как CD-ROM для ISO-образов. |
| [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) | Подключается в режиме для чтения, или как CD-ROM для ISO-образов. |
| [VirtualDisk](/modules/virtualization/cr.html#virtualdisk)                 | Подключается в режиме для чтения и записи.                        |

Подключение устройств возможно двумя способами:

- через спецификацию ВМ ([`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs)) — диски указываются в конфигурации [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) и для них задаётся порядок загрузки (по позиции в списке или через поле `bootOrder`). Рекомендуется при настройке ВМ вручную, а также когда нужен контроль порядка загрузки (например, ISO для установки ОС).
- через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (`vmbda`) — диск подключается отдельным ресурсом и не участвует в порядке загрузки. Диски подключаются через шину `virtio-scsi`, независимо от значения `enableParavirtualization`. Рекомендуется для автоматизации и когда нет прав на редактирование ВМ.

При `enableParavirtualization: true` оба способа позволяют подключать и отключать диски у работающей ВМ без перезагрузки, если диск доступен на узле, где она выполняется. При `enableParavirtualization: false` состав [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) у запущенной ВМ меняется только после перезагрузки; без перезагрузки диски можно подключать и отключать через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (`vmbda`).

{% alert level="warning" %}
Когда паравиртуализация выключена (`enableParavirtualization: false`), устройства из [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) работают на шине SATA, а у типа ОС `Legacy` — на шине IDE. У работающей ВМ изменения этого списка, включая подключение и отключение ISO-образа, вступают в силу только после перезагрузки.

Диски, подключённые через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment), используют шину `virtio-scsi` и подключаются без перезагрузки, если в гостевой ОС есть драйвер этой шины. Для типа ОС `Legacy` с выключенной паравиртуализацией такая привязка отклоняется, потому что шина IDE подключение на ходу не поддерживает. Добавьте устройство в [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) и перезапустите ВМ.
{% endalert %}

Подключить диск к работающей ВМ можно только тогда, когда хранилище доступно на том узле кластера, где выполняется виртуальная машина. При создании и обновлении ВМ, а также при создании [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment), учитываются правила размещения (`nodeSelector`, `affinity`, `tolerations`) тома, виртуальной машины и класса ВМ, и хотя бы одно общее допустимое размещение у них должно найтись. Если ВМ уже запущена и работает на конкретном узле, новый диск должен быть доступен на этом узле.

Пока живая миграция машины готовит целевой узел, диск нельзя ни подключить, ни отключить. Новый ресурс [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) остаётся в фазе `Pending` с причиной `BlockedByMigration` в условии `Attached`, а удаляемый остаётся в фазе `Terminating`, и оба доводятся до конца после миграции. Пока миграция стоит в очереди и целевой узел ещё не готовится, подключение и отключение работают как обычно.

## Подключение через спецификацию ВМ

Устройства, перечисленные в спецификации машины, подключаются при её запуске и остаются на месте всё время работы.

{% tabs bd-spec %}

{% tab "В командной строке" %}

Список блочных устройств задаётся в поле [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

Порядок загрузки по умолчанию совпадает с порядком устройств в списке, а задать его явно позволяет необязательное поле `bootOrder` (меньшее значение — выше приоритет). Если `bootOrder` указан хотя бы у одного устройства, в цепочку загрузки попадают только устройства с заданным `bootOrder`. Допустимы целые числа от 1 и выше, уникальные в пределах списка. При удалении устройства из списка порядок загрузки пересчитывается для оставшихся устройств.

Изменение порядка устройств в списке или значений `bootOrder` вступает в силу после перезагрузки ВМ. Например, можно подключить ISO-образ для установки ОС с нужным приоритетом загрузки, а после установки удалить его из списка. Если у ВМ отключена паравиртуализация (`enableParavirtualization: false`), правки в [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) у работающей ВМ, в том числе с ISO-образом, применяются после перезагрузки ВМ.

Фрагмент конфигурации виртуальной машины с блочными устройствами и явным порядком загрузки:

```yaml
spec:
  blockDeviceRefs:
    - kind: VirtualDisk
      name: <VD_NAME>
      bootOrder: 1
    - kind: VirtualImage
      name: <VI_NAME>
      bootOrder: 2
```

Для подключения диска к работающей виртуальной машине добавьте его в список [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs):

```yaml
spec:
  blockDeviceRefs:
    - kind: VirtualDisk
      name: <VD_NAME>
    - kind: VirtualImage
      name: <VI_NAME>
    - kind: VirtualDisk
      name: <ADDITIONAL_DISK_NAME>
```

Для отключения диска удалите его из списка. При `enableParavirtualization: false` изменение списка у запущенной ВМ вступит в силу после перезагрузки ВМ.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную ВМ и нажмите на её имя.
1. На вкладке «Конфигурация» прокрутите страницу до раздела «Диски».
1. В списке дисков доступны следующие действия:
   - «Добавить» — подключить к ВМ новый диск или образ;
   - «Извлечь» — отключить устройство от ВМ (образ или диск остаётся в проекте, его можно снова подключить к этой или другой ВМ);
   - «Удалить» — удалить сам ресурс образа или диска из кластера (после удаления его нельзя использовать повторно);
   - изменить размер диска — по значку карандаша рядом с текущим размером;
   - изменить порядок загрузки — изменив позицию диска в списке.

{% endtab %}

{% endtabs %}

## Подключение через VirtualMachineBlockDeviceAttachment

Отдельный ресурс подключает устройство к машине, не затрагивая её спецификацию.

{% tabs bd-vmbda %}

{% tab "В командной строке" %}

Ресурс [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) подключает и отключает блочное устройство у ВМ без изменения её спецификации. Подходит для автоматизации и сценариев, когда у пользователя нет прав на редактирование ВМ.

Создайте ресурс, который подключит пустой диск `blank-disk` к виртуальной машине `linux-vm`:

```shell
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-blank-disk
spec:
  blockDeviceRef:
    kind: VirtualDisk
    name: blank-disk
  virtualMachineName: linux-vm
EOF
```

Устройство подключено, когда ресурс переходит в фазу `Attached`. Остальные фазы описаны в поле [`.status.phase`](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment-v1alpha2-status-phase), а причину задержки показывает блок [`.status.conditions`](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment-v1alpha2-status-conditions).

Проверьте состояние вашего ресурса:

```bash
d8 k get vmbda attach-blank-disk
```

Пример вывода:

```console
NAME                PHASE      VIRTUALMACHINE   AGE
attach-blank-disk   Attached   linux-vm         3m7s
```
{: .nowrap-default }

Подключитесь к виртуальной машине и удостоверитесь, что диск подключён:

```bash
d8 v ssh cloud@linux-vm --command "lsblk"
```

Пример вывода:

```console
NAME    MAJ:MIN RM  SIZE RO TYPE MOUNTPOINTS
sda       8:0    0   10G  0 disk <--- статично подключенный диск linux-vm-root
|-sda1    8:1    0  9.9G  0 part /
|-sda14   8:14   0    4M  0 part
`-sda15   8:15   0  106M  0 part /boot/efi
sdb       8:16   0    1M  0 disk <--- cloudinit
sdc       8:32   0 95.9M  0 disk <--- динамически подключенный диск blank-disk
```
{: .nowrap-default }

Для отключения диска от виртуальной машины удалите ранее созданный ресурс:

```bash
d8 k delete vmbda attach-blank-disk
```

Образы подключаются так же, только в поле `kind` указывается значение [VirtualImage](/modules/virtualization/cr.html#virtualimage) или [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage).

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineBlockDeviceAttachment
metadata:
  name: attach-ubuntu-iso
spec:
  blockDeviceRef:
    kind: VirtualImage # или ClusterVirtualImage
    name: ubuntu-iso
  virtualMachineName: linux-vm
EOF
```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную ВМ и нажмите на её имя.
1. На вкладке «Конфигурация» прокрутите страницу до раздела «Диски».
1. В списке дисков доступны следующие действия:
   - «Добавить» — подключить к ВМ новый диск или образ; чтобы устройство подключилось как дополнительное (через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment), без перезагрузки ВМ), в окне «Диски / Образы» установите флажок «Дополнительный»;
   - «Извлечь» — отключить устройство от ВМ (образ или диск остаётся в проекте, его можно снова подключить к этой или другой ВМ);
   - «Удалить» — удалить сам ресурс образа или диска из кластера (после удаления его нельзя использовать повторно);
   - изменить размер диска — по значку карандаша рядом с текущим размером.

{% endtab %}

{% endtabs %}

## Именование дисков в гостевой ОС

Имена дисков в гостевой системе не постоянны, поэтому опираться на них в настройках опасно.

{% alert level="warning" %}
Имена блочных устройств (`/dev/sda`, `/dev/sdb`, `/dev/sdc` и так далее) присваиваются ядром Linux в порядке обнаружения устройств при загрузке. Этот порядок может меняться между перезагрузками, поэтому имена устройств могут измениться даже при неизменных SCSI-адресах.

Если использовать `/dev/sdX` в конфигурационных файлах (например, `/etc/fstab`) или скриптах, после перезагрузки ВМ можно смонтировать не тот диск или получить некорректную работу системы.
{% endalert %}

**Пример:**

После первой загрузки ВМ:

```console
$ lsscsi
[0:0:0:1]  disk    QEMU     QEMU HARDDISK   /dev/sda
[0:0:0:2]  disk    QEMU     QEMU HARDDISK   /dev/sdb
```
{: .nowrap-default }

После перезагрузки ВМ:

```console
$ lsscsi
[0:0:0:1]  disk    QEMU     QEMU HARDDISK   /dev/sdb
[0:0:0:2]  disk    QEMU     QEMU HARDDISK   /dev/sda
```
{: .nowrap-default }

SCSI-адреса (`0:0:0:1`, `0:0:0:2`) остаются неизменными, но имена устройств (`/dev/sda`, `/dev/sdb`) меняются местами.

Используйте стабильные идентификаторы вместо `/dev/sdX`:

- `/dev/disk/by-uuid/` — по UUID разделов (предпочтительно для `/etc/fstab`);
- `/dev/disk/by-path/` — по SCSI пути подключения;
- `/dev/disk/by-id/` — по SCSI ID устройства.

В конфигурационных файлах и скриптах используйте UUID разделов или символические ссылки из `/dev/disk/by-*` вместо имён `/dev/sdX`.

## Именование сетевых интерфейсов в гостевой ОС

В системах без поддержки предсказуемого именования интерфейсов (predictable network interface naming) имена сетевых интерфейсов (`eth0`, `eth1`, `eth2` и так далее) присваиваются ядром Linux в порядке обнаружения устройств при загрузке. При добавлении новых сетевых интерфейсов или изменении порядка сетей в [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) порядок интерфейсов может измениться, из-за чего IP-адреса могут быть назначены не тем интерфейсам.

Использование `ethX` в конфигурационных файлах (например, `/etc/network/interfaces`, `netplan`, `systemd-networkd`) или в скриптах при добавлении новых интерфейсов или изменении порядка сетей может привести к сбоям в работе сети или подключению к неверной сети.

В современных дистрибутивах с systemd (Ubuntu 16.04+, Debian 9+, CentOS 7+, RHEL 7+) по умолчанию используются предсказуемые имена интерфейсов (`enpXsY`, `ensX`, `enoX`), которые основаны на физических характеристиках устройства (PCI координаты) и остаются стабильными между перезагрузками и при добавлении новых интерфейсов.

Но даже с предсказуемыми именами привязывайте конфигурацию сети к MAC-адресам интерфейсов, особенно если порядок сетей меняется в [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) или добавлении новых интерфейсов.

Пример для систем без предсказуемого именования:

Изначально ВМ имеет два интерфейса:

```console
$ ip link show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500
3: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500
```
{: .nowrap-default }

После добавления нового интерфейса в начало списка [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) и перезагрузки ВМ:

```console
$ ip link show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500  # Новый интерфейс
3: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500  # Старый eth0
4: eth2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500  # Старый eth1
```
{: .nowrap-default }

MAC-адреса остаются неизменными, но имена интерфейсов (`eth0`, `eth1`) сдвигаются, что может привести к назначению IP-адресов не тем интерфейсам.

Используйте стабильные идентификаторы вместо `ethX`:

- `enpXsY` — предсказуемые имена на основе физического расположения (systemd networkd naming scheme, включены по умолчанию в современных системах);
- привязка по MAC-адресу — в конфигурации `netplan`, `systemd-networkd` или `/etc/network/interfaces` (предпочтительно для гарантированной стабильности).

В конфигурационных файлах и скриптах используйте стабильные имена интерфейсов (`enpXsY`) или привязку по MAC-адресу вместо имён `ethX`.

{% alert level="info" %}
Предсказуемый порядок интерфейсов соблюдается только в гостевых ОС с systemd (например, Ubuntu, Debian). В Alpine и других дистрибутивах без systemd порядок может не совпадать.
{% endalert %}

Чтобы открыть приложение машины другим машинам или пользователям снаружи кластера, настройте сервис или Ingress, как описано в разделе [«Доступ к приложениям на виртуальной машине»](../network/virtualization/vm-publishing.html).
