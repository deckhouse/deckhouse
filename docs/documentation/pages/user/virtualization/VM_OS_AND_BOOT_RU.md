---
title: "Гостевая ОС и загрузчик виртуальной машины"
permalink: ru/user/virtualization/vm-os-and-boot.html
description: "Тип гостевой операционной системы и загрузчик виртуальной машины: BIOS, UEFI, Secure Boot и особенности загрузки Windows."
search: тип ОС, загрузчик, UEFI, BIOS, Secure Boot, osType
lang: ru
---

Параметр `osType` определяет тип операционной системы и применяет оптимальный набор виртуальных устройств и параметров для корректной работы ВМ.

Поддерживаемые значения:

- `Generic` (по умолчанию) — для Linux и других операционных систем. Используется стандартная конфигурация виртуальных устройств.
- `Windows` — для операционных систем семейства Microsoft Windows. Автоматически включает функции Hyper-V, TPM-устройство и другие настройки, оптимизированные для работы Windows.
- `Legacy` — для операционных систем без встроенных драйверов AHCI и virtio, то есть Windows XP, Windows 2000, Windows Server 2003, систем эпохи DOS и Linux с ядром старше 2.6.19. Такая ВМ получает чипсет i440fx, а с `enableParavirtualization: false` — ещё и шину IDE для дисков и CD-ROM и сетевой адаптер RTL8139, драйверы которых есть в этих операционных системах.

{% alert level="warning" %}
Виртуальная машина получает эмулированный TPM, состояние которого хранится в памяти и не сохраняется. При перезагрузке или миграции ВМ состояние TPM сбрасывается. Учитывайте это ограничение при использовании функций безопасности Windows, зависящих от TPM.
{% endalert %}

Набор виртуальных устройств, которые видит гостевая ОС:

| Устройство                     | `Generic`                                                   | `Windows`                                                   | `Legacy`                     |
| ------------------------------ | ----------------------------------------------------------- | ----------------------------------------------------------- | ---------------------------- |
| Чипсет                         | q35                                                         | q35                                                         | i440fx                       |
| Загрузчик                      | `BIOS`, `EFI`, `EFIWithSecureBoot`                          | `BIOS`, `EFI`, `EFIWithSecureBoot`                          | только `BIOS`                |
| Шина дисков                    | virtio-scsi, при `enableParavirtualization: false` — SATA   | virtio-scsi, при `enableParavirtualization: false` — SATA   | virtio-blk, при `enableParavirtualization: false` — IDE |
| Шина CD-ROM                    | virtio-scsi, при `enableParavirtualization: false` — SATA   | virtio-scsi, при `enableParavirtualization: false` — SATA   | IDE |
| Блочных устройств в [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) | не более 16                                    | не более 16                                                 | не более 16, при `enableParavirtualization: false` — не более 4 |
| Сетевой адаптер                | virtio-net, при `enableParavirtualization: false` — e1000   | virtio-net, при `enableParavirtualization: false` — e1000   | virtio-net, при `enableParavirtualization: false` — RTL8139 |
| USB-контроллер                 | xHCI (USB 3.0)                                              | xHCI (USB 3.0)                                              | UHCI (USB 1.1)               |
| TPM                            | нет                                                         | TPM 2.0                                                     | нет                          |
| Генератор случайных чисел      | virtio-rng                                                  | нет                                                         | нет                          |
| Функции Hyper-V                | нет                                                         | да                                                          | нет                          |
| Подключение дисков на ходу     | да                                                          | да                                                          | только при `enableParavirtualization: true` и только если у гостевой ОС есть драйвер virtio-scsi |
| Изменение CPU и памяти на ходу | да                                                          | да                                                          | нет                          |

Проброс USB-устройств для `Legacy` работает, но контроллер UHCI ограничен скоростью USB 1.1 в 12 Мбит/с, поэтому быстрый накопитель в такой ВМ упрётся в шину.

Выбирайте `Legacy`, когда гостевая операционная система не умеет работать с контроллером AHCI, а не просто потому, что она старая. Для Linux с ядром 2.6.19 и новее подходит `osType: Generic` с `enableParavirtualization: false` — там нет ограничения в четыре устройства и диски можно подключать на ходу через [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment).

{% alert level="warning" %}
Для `Legacy` недоступны:

- изменение числа ядер процессора и объёма памяти на работающей ВМ, потому что эти гостевые ОС не вводят их в работу. Изменение принимается, ВМ показывает его в [`.status.restartAwaitingChanges`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-restartawaitingchanges) вместе с условием `AwaitingRestartToApplyConfiguration`, а применяется оно после перезапуска;
- изменение состава [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) у работающей ВМ, потому что в обоих режимах паравиртуализации эти диски остаются статическими, поэтому блочное устройство добавляйте до запуска ВМ;
- загрузчики `EFI` и `EFIWithSecureBoot`, а также начальная инициализация (`cloud-init` и Sysprep), потому что эти гостевые ОС их не поддерживают;
- сведения о гостевой ОС в статусе ВМ и информация о файловых системах.

Гостевой агент QEMU в такую ОС поставить можно, из архивного выпуска virtio-win, и ВМ действительно покажет `AgentReady`, но его версия слишком старая для модуля. У ВМ появляется условие `AgentVersionNotSupported`, сведения о гостевой ОС не собираются, а обновить агента не на что.

Снимок с `requiredConsistency: true` тоже не завершится успешно, но по другой причине. Модуль запрашивает заморозку файловой системы, а агент отвечает, что команда в его сборке отключена, сообщением `guest-fsfreeze-status has been disabled for this instance`. На Windows заморозка идёт через провайдер VSS, которого в этой сборке нет. Снимок около десяти минут ждёт в фазе `InProgress` и переходит в `Failed`, поэтому для таких ВМ задавайте `requiredConsistency: false`.

С `enableParavirtualization: false` добавляется ещё одно ограничение. Блочных устройств может быть не более четырёх суммарно, потому что шина IDE предоставляет два канала по два устройства.
{% endalert %}

Пример конфигурации для виртуальной машины с Windows XP:

```yaml
spec:
  osType: Legacy
  bootloader: BIOS
  enableParavirtualization: false
  # остальные параметры...
```

Параметр `bootloader` определяет тип загрузчика виртуальной машины:

- `BIOS` (по умолчанию) — использование устаревшего BIOS;
- `EFI` — использование Unified Extensible Firmware Interface (UEFI/EFI);
  - `EFIWithSecureBoot` — использование UEFI/EFI с поддержкой Secure Boot.

Пример конфигурации для виртуальной машины с Windows:

```yaml
spec:
  osType: Windows
  bootloader: EFI
  # остальные параметры...
```

Пример конфигурации для виртуальной машины с Linux (значения по умолчанию можно не указывать):

```yaml
spec:
  osType: Generic
  bootloader: BIOS
  # остальные параметры...
```

{% alert level="info" %}
Для современных Linux-дистрибутивов выбирайте `bootloader: EFI`, а для Windows — `bootloader: EFI` или `bootloader: EFIWithSecureBoot`.
{% endalert %}

{% alert level="warning" %}
Для `EFIWithSecureBoot` нужен постоянный том под состояние Secure Boot, а для его создания — StorageClass по умолчанию в кластере. Если его нет, виртуальная машина не запускается и остаётся в состоянии `Pending`, а в её статусе указывается, что StorageClass по умолчанию не найден. Как только StorageClass по умолчанию появится, машина запустится автоматически.
{% endalert %}

Параметр `enableParavirtualization` управляет использованием шины `virtio` для подключения виртуальных устройств ВМ. Изменение значения параметра учитывается только после перезагрузки ВМ.

- `true` (по умолчанию) — используется шина `virtio` для дисков, сетевых интерфейсов и других устройств, что обеспечивает лучшую производительность. Состав [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) у работающей ВМ можно менять без перезагрузки — добавляя и удаляя устройства, если диск доступен на узле, где выполняется ВМ.
- `false` — используется эмуляция стандартных устройств (SATA для дисков, e1000 для сетевых интерфейсов; IDE и RTL8139 для типа ОС `Legacy`), что может быть необходимо для совместимости со старыми ОС без драйверов `VirtIO`. Изменения в [`.spec.blockDeviceRefs`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-blockdevicerefs) на работающей ВМ (добавление и удаление дисков и образов, в том числе ISO) вступают в силу после перезагрузки ВМ. Подключать и отключать диски без перезагрузки можно через ресурс [VirtualMachineBlockDeviceAttachment](/modules/virtualization/cr.html#virtualmachineblockdeviceattachment) (`vmbda`), не меняя список в спецификации ВМ.

{% alert level="info" %}
Для использования режима паравиртуализации (`virtio`) в некоторых операционных системах требуется установка соответствующих драйверов. Если драйверы не установлены, ВМ может не загрузиться или устройства могут работать некорректно.
{% endalert %}

Для типа ОС `Legacy` значение по умолчанию `true` не подходит, потому что встроенных драйверов virtio у этих операционных систем нет, поэтому для ВМ, которую предстоит установить с оригинального носителя, укажите `enableParavirtualization: false` — иначе установщик сообщит, что не нашёл жёстких дисков. При создании и изменении такой ВМ выдаётся предупреждение.

Оставляйте `enableParavirtualization: true` для ВМ с типом ОС `Legacy` только тогда, когда драйверы virtio уже установлены в гостевой ОС. Тогда ВМ сохраняет чипсет i440fx и загрузчик `BIOS`, но получает диски на virtio-blk, адаптер virtio-net и лишается ограничения в четыре устройства. CD-ROM остаётся на шине IDE, потому что у virtio-blk привода нет.

Переключить уже установленную систему можно так:

1. Установите драйверы virtio в гостевой ОС. Для Windows XP, 2000 и Server 2003 возьмите архивный выпуск virtio-win, потому что в текущих выпусках драйверов для этих систем уже нет, и установите `viostor`, драйвер virtio-blk. Драйвера virtio-scsi для этих ОС в пакете нет.
1. Выключите ВМ.
1. Задайте `enableParavirtualization: true`.
1. Запустите ВМ.

Порядок шагов важен. Драйвер дискового контроллера должен появиться в гостевой ОС до переключения, а не после, ведь именно с этого контроллера ВМ загружается. Если после переключения ВМ не загрузилась, верните `enableParavirtualization: false` и перезапустите её. Диски вернутся на шину IDE, и гостевая ОС загрузится как прежде.

Пример конфигурации с отключенной паравиртуализацией:

```yaml
spec:
  enableParavirtualization: false
  # остальные параметры...
```
