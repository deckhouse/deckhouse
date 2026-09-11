---
title: "PCI-устройства в виртуальных машинах"
permalink: ru/admin/configuration/virtualization/pci-devices.html
description: "Проброс PCI-устройств в виртуальные машины: требования к узлам, ресурсы NodePCIDevice и PCIDevice, назначение устройства неймспейсу, ограничения."
search: PCI-устройства, проброс PCI, NodePCIDevice, PCIDevice, vfio-pci, IOMMU
lang: ru
---

{% alert level="warning" %}
Проброс PCI-устройств доступен в коммерческих редакциях DP.
{% endalert %}

Проброс PCI-устройств позволяет подключить к виртуальной машине (ВМ) физическое устройство узла, например промышленный контроллер, аппаратный модуль безопасности, плату видеозахвата, ПЛИС или сетевую карту целиком. В гостевой операционной системе такое устройство работает под её собственным драйвером, поэтому в ВМ можно использовать оборудование, которое модуль не поддерживает напрямую.

Устройство попадает в машину в два шага. Сначала вы назначаете его неймспейсу проекта, а затем владелец проекта указывает устройство в спецификации своей машины. Устройство предоставляется в монопольное использование, поэтому доступно только в одном неймспейсе и только одной машине.

Драйверы устройства модуль переключает сам, вручную настраивать их на узле не требуется. При запуске машины модуль отвязывает устройство от штатного драйвера ядра и привязывает его к драйверу `vfio-pci`, а после остановки машины возвращает штатному драйверу. Пока машина работает, узел это устройство не использует.

## Требования к узлам

За проброс PCI-устройств отвечает системный компонент `virtualization-dra-pci`. Он запускается только на узлах с containerd версии 2 и включённой аппаратной виртуализацией ввода-вывода. Узлы модуль проверяет сам и назначает подходящим лейбл `virtualization.deckhouse.io/vfio=true`.

Чтобы включить аппаратную виртуализацию ввода-вывода, задайте в BIOS узла `VT-d` у Intel или `AMD-Vi` у AMD и добавьте в параметры ядра `intel_iommu=on` или `amd_iommu=on`.

Чтобы посмотреть, какие узлы готовы к пробросу PCI-устройств, выполните команду:

```bash
d8 k get nodes -l virtualization.deckhouse.io/vfio=true,virtualization.deckhouse.io/containerd-version=v2
```

Пример вывода:

```console
NAME     STATUS   ROLES    AGE   VERSION
node-1   Ready    worker   10d   v1.34.1
```

Узел, отсутствующий в выводе, лейбл не получил. Проверьте на нём каталог `/sys/kernel/iommu_groups`. Пустой каталог означает, что аппаратная виртуализация ввода-вывода выключена. Включите её и перезагрузите узел, и в течение нескольких минут лейбл будет назначен автоматически.

Чтобы убедиться, что компонент действительно работает на этих узлах, выполните команду:

```bash
d8 k -n d8-virtualization get pods -l app=virtualization-dra-pci -o wide
```

## Назначение неймспейса PCI-устройству

Модуль обнаруживает устройства на подходящих узлах и создаёт для каждого из них ресурс [NodePCIDevice](/modules/virtualization/cr.html#nodepcidevice). Шину PCI компонент сканирует при запуске и далее каждые пять минут, поэтому только что установленное устройство появляется в списке через несколько минут.

Чтобы предоставить устройство проекту, выполните следующие шаги.

1. Найдите устройство среди обнаруженных:

   ```bash
   d8 k get nodepcidevice
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                            NODE     ADDRESS        READY   ASSIGNED   ATTACHED   NAMESPACE    AGE
   pci-4f2c0b1e8d9a3c5b7e1f0a2d4c6b8e0f1a3c5d7e    node-1   0000:3b:00.0   True    False      False                   10m
   pci-9a1b3c5d7e9f0a2b4c6d8e0f1a3b5c7d9e1f0a2b    node-2   0000:65:00.0   True    True       False      my-project   15m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Имя ресурса формируется как хеш от параметров устройства и имени узла, поэтому ищите нужное устройство по колонкам `NODE` и `ADDRESS`. Проверить выбор помогает блок [`.status.attributes`](/modules/virtualization/cr.html#nodepcidevice-v1alpha2-status-attributes), где указаны адрес на шине PCI и идентификаторы производителя и модели. По ним то же устройство находится в выводе команды `lspci -nn` на узле.

1. Назначьте неймспейс параметром [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodepcidevice-v1alpha2-spec-assignednamespace):

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: NodePCIDevice
   metadata:
     name: pci-4f2c0b1e8d9a3c5b7e1f0a2d4c6b8e0f1a3c5d7e
   spec:
     assignedNamespace: my-project
   EOF
   ```

1. Убедитесь, что в неймспейсе появился ресурс [PCIDevice](/modules/virtualization/cr.html#pcidevice):

   ```bash
   d8 k get pcidevice -n my-project
   ```

После этого владелец проекта подключает устройство к своей машине, указав имя ресурса [PCIDevice](/modules/virtualization/cr.html#pcidevice) в параметре [`.spec.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-pcidevices) ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).

Чтобы сделать устройство недоступным проекту, очистите параметр [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodepcidevice-v1alpha2-spec-assignednamespace) или укажите другой неймспейс. Ресурс [PCIDevice](/modules/virtualization/cr.html#pcidevice) в прежнем неймспейсе удаляется.

Пока устройство указано в спецификации машины, ресурс [PCIDevice](/modules/virtualization/cr.html#pcidevice) не удаляется, а сама машина продолжает работать. Попросите владельца проекта убрать устройство из спецификации. Если вместо очистки параметра вы указали другой неймспейс, ресурс появится в нём сразу, но машина нового проекта останется в фазе `Pending`, пока устройство занято прежней машиной.

## Требования и ограничения PCI-устройств

Планируя проброс PCI-устройств, учитывайте следующие требования и ограничения:

- проброс работает в кластере с [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) версии не ниже 1.34 и containerd версии 2 на узлах, а в kube-apiserver должны быть включены feature gates `DRAResourceClaimDeviceStatus`, `DRADeviceBindingConditions` и `DRAConsumableCapacity`;
- модуль обнаруживает не все устройства узла, потому что оборудование, от которого зависит работа самого узла, остаётся в его распоряжении. В списке не появятся:
  - видеоадаптеры, которыми занимается модуль GPU;
  - устройства, интегрированные в чипсет, мосты, контроллеры памяти и системная периферия;
  - сетевые контроллеры с активными интерфейсами;
  - контроллеры накопителей, диски которых использует узел;
  - устройства, попавшие с ними в одну IOMMU-группу, потому что в ВМ группа передаётся целиком;
- ВМ запускается только на том узле, где находится подключённое к ней устройство, поэтому все её PCI-устройства должны быть на одном узле, иначе спецификация будет отклонена;
- ВМ с PCI-устройством живой миграцией не переносится, а условие `Migratable` получает причину `VirtualMachineHostDevicesNotMigratable`, поэтому перед выводом узла на обслуживание такие машины нужно остановить;
- устройства подключаются при запуске ВМ, поэтому изменение параметра [`.spec.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-pcidevices) требует её перезапуска;
- устройство подключается только к одной ВМ, а к каждой ВМ подключается не более восьми устройств;
- проброшенная сетевая карта работает в обход сетевой подсистемы кластера. Она не отражается в параметре [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks), а адреса IP и MAC на ней модуль не выдаёт и не учитывает.
