---
title: "USB-устройства в виртуальных машинах"
permalink: ru/admin/configuration/virtualization/usb-devices.html
description: "Проброс USB-устройств в виртуальные машины: подготовка узлов, ресурс NodeUSBDevice, назначение устройства проекту, требования и ограничения."
search: USB-устройства, проброс USB, NodeUSBDevice, usbip
lang: ru
---

{% alert level="warning" %}
Проброс USB-устройств доступен в коммерческих редакциях DP.
{% endalert %}

За проброс USB-устройств к виртуальным машинам (ВМ) отвечает системный компонент `virtualization-dra`, которому на узле нужны три модуля ядра:

- `usbip_core`;
- `usbip_host`;
- `vhci_hcd`.

Модуль загружает их на узлах сам. Узел, где доступны все три модуля, получает лейбл `virtualization.deckhouse.io/usbip=true`, и только на таких узлах запускается компонент `virtualization-dra`. Если модули ядра перестают быть доступны, лейбл снимается, а компонент с узла удаляется.

Чтобы посмотреть, какие узлы готовы к пробросу USB-устройств, выполните команду:

```bash
d8 k get nodes -l virtualization.deckhouse.io/usbip=true
```

Пример вывода:

```console
NAME     STATUS   ROLES    AGE   VERSION
node-1   Ready    worker   10d   v1.34.1
```

Чтобы убедиться, что компонент действительно работает на этих узлах, выполните команду:

```bash
d8 k -n d8-virtualization get pods -l app=virtualization-dra -o wide
```

Узел, которого нет в выводе, загрузить модули ядра не смог, и USB-устройства этого узла не обнаруживаются. Установите модули ядра самостоятельно из пакета вашей операционной системы или соберите их для используемого ядра. Модуль обнаружит их сам и в течение нескольких минут назначит узлу лейбл.

## Путь USB-устройства от узла до машины

Путь USB-устройства от узла до виртуальной машины состоит из четырёх шагов:

1. DRA-драйвер обнаруживает USB-устройства на узлах и публикует сведения о них в API Kubernetes как [ResourceSlice](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/). Контроллер модуля создаёт ресурсы [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) по этим данным.

1. Администратор назначает неймспейс ресурсу [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice), задав параметр [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodeusbdevice-v1alpha2-spec-assignednamespace). Это делает устройство доступным в этом неймспейсе.

1. После назначения неймспейса контроллер модуля создаёт в нём ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice).

1. Владелец проекта подключает устройство [USBDevice](/modules/virtualization/cr.html#usbdevice) к виртуальной машине, добавив его в параметр [`.spec.usbDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-usbdevices) ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

## Обнаруженные устройства (NodeUSBDevice)

Ресурс [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) описывает физическое USB-устройство, обнаруженное на узле. Ресурс существует на уровне кластера, поэтому все обнаруженные устройства видны вам в одном списке:

```bash
d8 k get nodeusbdevice
```

Пример вывода:

```console
NAME              NODE     READY   ASSIGNED   ATTACHED   NAMESPACE    AGE
usb-flash-drive   node-1   True    False      False                   10m
logitech-webcam   node-2   True    True       True       my-project   15m
```
{: .nowrap-default }

Готовность устройства и его состояние отражают условия в блоке [`.status.conditions`](/modules/virtualization/cr.html#nodeusbdevice-v1alpha2-status-conditions). Условия `Ready` и `Attached` совпадают с [условиями USBDevice](../../../user/virtualization/vm-devices.html#условия-usbdevice), а условие `Assigned` показывает, назначен ли устройству неймспейс:

- `Available` — неймспейс не назначен;
- `InProgress` — неймспейс назначен, и ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) создаётся;
- `Assigned` — ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) создан, устройство доступно в неймспейсе.

### Назначение неймспейса USB-устройству

Пока устройству не назначен неймспейс, владелец проекта его не видит. Чтобы сделать устройство доступным в проекте, выполните следующие шаги.

1. Подключите USB-устройство к узлу, готовому к пробросу, и дождитесь появления ресурса [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice).

1. Назначьте неймспейс параметром [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodeusbdevice-v1alpha2-spec-assignednamespace):

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: NodeUSBDevice
   metadata:
     name: logitech-webcam
   spec:
     assignedNamespace: my-project
   EOF
   ```

1. Убедитесь, что в неймспейсе появился ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice):

   ```bash
   d8 k get usbdevice -n my-project
   ```

После этого владелец проекта подключает устройство к виртуальной машине.

## Просмотр информации об USB-устройстве

Полные сведения об устройстве и его текущее состояние доступны в статусе ресурса.

{% tabs usb-view %}

{% tab "В командной строке" %}

Идентификаторы устройства, его расположение и текущие условия хранятся в статусе ресурса:

```bash
d8 k get nodeusbdevice <DEVICE_NAME> -o yaml
```

Здесь `<DEVICE_NAME>` — имя ресурса [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice).

Чтобы получить только атрибуты устройства, обратитесь к нужным полям напрямую:

```bash
d8 k get nodeusbdevice <DEVICE_NAME> \
  -o jsonpath='{.status.attributes.manufacturer}{" "}{.status.attributes.product}{" ("}{.status.attributes.vendorID}{":"}{.status.attributes.productID}{")\n"}'
```

Пример вывода:

```console
Logitech Webcam C920 (046d:082d)
```

> Когда устройство физически отключают от узла, условие `Attached` принимает значение `False`, а условие `Ready` получает причину `NotFound`. То же самое отражается в статусе ресурса [USBDevice](/modules/virtualization/cr.html#usbdevice) в проектном неймспейсе.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Система», далее в раздел «Виртуализация» → «USB-устройства узлов».
1. Посмотрите список, в котором показаны статус устройства, производитель, продукт, серийный номер, узел, шина, номер устройства и назначенный неймспейс.

{% endtab %}

{% endtabs %}

Устройства, назначенные проекту, его владелец видит в разделе «Виртуализация» → «USB-устройства» своего проекта.

## Требования и ограничения

Планируя проброс USB-устройств, учитывайте следующие требования и ограничения:

- узел, на котором нужно обнаруживать USB-устройства, должен нести лейбл `virtualization.deckhouse.io/usbip=true` и работать на containerd версии 2, иначе компонент `virtualization-dra` там не запустится;
- устройство передаётся виртуальной машине по сети средствами USBIP, поэтому ВМ может работать на другом узле, а не на том, куда устройство подключено физически;
- пробросить можно только устройство, которое сообщает о себе скорость USB 2.0 (480 Мбит/с) или USB 3.x (от 5 Гбит/с). Устройство с меньшей скоростью, например мышь или клавиатуру на 1,5 или 12 Мбит/с, модуль подключить к ВМ не даст;
- узел подключает не более 16 устройств, по 8 на концентратор USB 2.0 и USB 3.0;
- концентратор выбирается по скорости устройства, и вручную его не изменить. Устройство со скоростью USB 2.0 к концентратору USB 3.0 не подключится, как и наоборот;
- устройство можно подключать к работающей ВМ и отключать от неё, не останавливая ВМ.
