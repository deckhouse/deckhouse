---
title: "USB-устройства"
permalink: ru/virtualization-platform/documentation/user/resource-management/usb-devices.html
lang: ru
---

{% alert level="warning" %}
Проброс USB-устройств доступен только в **Enterprise Edition (EE)** платформы Deckhouse Virtualization Platform.
{% endalert %}

DVP поддерживает проброс USB-устройств в виртуальные машины с использованием DRA (Dynamic Resource Allocation). В этом разделе описано, как использовать USB-устройства с виртуальными машинами.

Для проброса USB требуются:

- `containerd v2` — подробные требования к узлам кластера описаны в параметре [`defaultCRI`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri);
- [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) версии не ниже 1.34;
- [Deckhouse Kubernetes Platform (DKP)](https://releases.deckhouse.ru/) версии не ниже 1.75.

## Обзор

DVP предоставляет два кастомных ресурса для управления USB-устройствами:

- [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) (cluster-wide-ресурс) — представляет USB-устройство, обнаруженное на конкретном узле.
- [USBDevice](/modules/virtualization/cr.html#usbdevice) (namespaced-ресурс) — представляет USB-устройство, доступное для подключения к виртуальным машинам в заданном неймспейсе.

## Принцип работы

Проброс USB-устройства проходит через последовательный жизненный цикл — от обнаружения устройства на узле до подключения к виртуальной машине:

1. DRA-драйвер обнаруживает USB-устройства на узлах и публикует сведения о них в API Kubernetes как ResourceSlice. Контроллер модуля создаёт ресурсы [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) по этим данным.

1. Администратор назначает неймспейс ресурсу [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice), задав параметр ресурса `.spec.assignedNamespace`. Это делает устройство доступным в этом неймспейсе.

1. После назначения неймспейса контроллер модуля создаёт в нём ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice).

1. Устройство [USBDevice](/modules/virtualization/cr.html#usbdevice) подключается к виртуальной машине путём добавления в параметр ресурса `.spec.usbDevices` ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

## Быстрый старт

Следующие шаги описывают минимальный сценарий подключения USB-устройства к виртуальной машине:

1. Подключите USB-устройство к узлу кластера.
1. Убедитесь, что создан ресурс [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice):

   ```bash
   d8 k get nodeusbdevice
   ```

1. Назначьте неймспейс ресурсу [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice), задав параметр ресурса `.spec.assignedNamespace`:

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

1. Убедитесь, что в целевом неймспейсе создан соответствующий ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice):

   ```bash
   d8 k get usbdevice -n my-project
   ```

1. Добавьте устройство в параметр ресурса `.spec.usbDevices` ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine):

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
   spec:
     # ... другие настройки ВМ ...
     usbDevices:
       - name: logitech-webcam
   EOF
   ```

## NodeUSBDevice

Ресурс [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) отражает состояние физического USB-устройства, обнаруженного на узле кластера. Это cluster-wide-ресурс, представляющий физическое USB-устройство на узле.

Пример просмотра всех обнаруженных USB-устройств:

```bash
d8 k get nodeusbdevice
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                 NODE           READY   ASSIGNED   NAMESPACE   AGE
usb-flash-drive     node-1         True    False                  10m
logitech-webcam     node-2         True    True      my-project   15m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

### Условия NodeUSBDevice

Состояние ресурса [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) описывается набором условий, которые отражают готовность устройства и факт назначения неймспейса:

- **Ready**: Указывает, готово ли устройство к использованию.
  - `Ready` — устройство готово к использованию;
  - `NotReady` — устройство существует, но не готово;
  - `NotFound` — устройство отсутствует на хосте.

- **Assigned**: Указывает, назначен ли неймспейс устройству.
  - `Assigned` — неймспейс назначен и ресурс USBDevice создан;
  - `Available` — для устройства не назначен неймспейс;
  - `InProgress` — подключение устройства к неймспейсу выполняется.

### Назначение неймспейса USB-устройству

Перед подключением USB-устройства к виртуальной машине его необходимо сделать доступным в конкретном неймспейсе. Для этого задайте параметр ресурса `.spec.assignedNamespace`:

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

После назначения неймспейса соответствующий ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) автоматически создаётся в указанном неймспейсе.

## USBDevice

[USBDevice](/modules/virtualization/cr.html#usbdevice) — это namespaced-ресурс, представляющий USB-устройство, доступное для подключения к виртуальным машинам в заданном неймспейсе. Появляется автоматически, когда у связанного [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) задан неймспейс в параметре ресурса `.spec.assignedNamespace`.

Пример просмотра USB-устройств в неймспейсе:

```bash
d8 k get usbdevice -n my-project
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME               NODE     MANUFACTURER   PRODUCT              SERIAL       ATTACHED   AGE
logitech-webcam    node-2   Logitech       Webcam C920         ABC123456   False      10m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

### Атрибуты USBDevice

Ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) содержит подробную информацию о физическом USB-устройстве. Атрибуты перечислены в `.status.attributes`:

- `vendorID` — USB идентификатор производителя (шестнадцатеричный формат);
- `productID` — USB идентификатор продукта (шестнадцатеричный формат);
- `bus` — номер USB-шины;
- `deviceNumber` — номер USB-устройства на шине;
- `serial` — серийный номер устройства;
- `manufacturer` — название производителя устройства;
- `product` — название продукта устройства;
- `name` — имя устройства.

### Условия USBDevice

Ресурс [USBDevice](/modules/virtualization/cr.html#usbdevice) содержит условия, отражающие готовность устройства и его состояние подключения:

- **Ready**: Указывает, готово ли устройство к использованию.
  - `Ready` — устройство готово к использованию;
  - `NotReady` — устройство существует, но не готово;
  - `NotFound` — устройство отсутствует на хосте.

- **Attached**: Указывает, подключено ли устройство к виртуальной машине.
  - `AttachedToVirtualMachine` — устройство подключено к ВМ;
  - `Available` — устройство доступно для подключения;
  - `NoFreeUSBIPPort` — устройство запрошено ВМ, но не может быть подключено, так как на целевом узле нет свободных USBIP-портов. В этом случае `Attached=False`.

## Подключение USB-устройства к ВМ

После появления ресурса [USBDevice](/modules/virtualization/cr.html#usbdevice) в неймспейсе его можно подключить к виртуальной машине. Для этого добавьте устройство в параметр ресурса `.spec.usbDevices` ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine):

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # ... другие настройки ВМ ...
  usbDevices:
    - name: logitech-webcam
EOF
```

После создания или обновления ВМ USB-устройство будет подключено к указанной виртуальной машине.

{% alert level="info" %}
Виртуальная машина должна быть размещена на том же узле, к которому физически подключено USB-устройство.
{% endalert %}

{% alert level="warning" %}
Во время миграции ВМ USB-устройство ненадолго отключится и подключится на новом узле в момент переключения ВМ. При сбое миграции устройство останется на старом узле.
{% endalert %}

## Просмотр информации об USB-устройстве

Для просмотра подробной информации об USB-устройстве:

```bash
d8 k describe nodeusbdevice <device-name>
```

Пример вывода:

```console
Name:         logitech-webcam
Namespace:
Labels:       <none>
Annotations:  <none>
API Version:  virtualization.deckhouse.io/v1alpha2
Kind:         NodeUSBDevice
Metadata:
  Creation Timestamp:  2024-01-15T10:30:00Z
  Generation:          1
  UID:                 abc123-def456-ghi789
Spec:
  Assigned Namespace:  my-project
Status:
  Node Name:           node-2
  Attributes:
    Bus:               1
    Device Number:     2
    Manufacturer:      Logitech
    Name:              Webcam C920
    Product:           Webcam C920
    Product ID:        082d
    Serial:            ABC123456
    Vendor ID:         046d
  Conditions:
    Type:              Ready
    Status:            True
    Reason:            Ready
    Message:           Device is ready to use
    Type:              Assigned
    Status:            True
    Reason:            Assigned
    Message:           Namespace is assigned for the device
  Observed Generation: 1
```

{% alert level="info" %}
Если USB-устройство физически отключено от узла, условие `Attached` принимает значение `False`.  
Статусы ресурсов `USBDevice` и `NodeUSBDevice` обновляются и указывают на отсутствие устройства на хосте.
{% endalert %}

## Требования и ограничения

При использовании проброса USB-устройств необходимо учитывать следующие требования и ограничения:

- Драйвер DRA должен быть установлен на узлах, где требуется обнаружение USB-устройств.
- USB-устройства пробрасываются на узел ВМ по сети с использованием USBIP. Виртуальная машина не обязана работать на том же узле, где физически подключено устройство. При подключении по сети действуют следующие ограничения по количеству устройств и выбору концентратора:
  - Узел может подключить не более 16 USB-устройств: до 8 на концентратор USB 2.0 и до 8 на концентратор USB 3.0.
  - Концентратор определяется скоростью устройства и не может быть выбран вручную. Устройство, работающее на USB 2.0, не может быть подключено к концентратору USB 3.0, и наоборот.
- USB-устройства поддерживают hot-plug — их можно подключать и отключать от работающей ВМ без её остановки.
- Для проброса USB-устройств требуются соответствующие модули ядра на узле.
