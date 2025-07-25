---
title: "Обзор"
permalink: ru/admin/configuration/storage/sds/node-configurator/about.html
lang: ru
---

Deckhouse Kubernetes Platform предоставляет автоматическое управление логическими томами (Logical Volume Manager, LVM) на узлах кластера с помощью пользовательских ресурсов Kubernetes. Эта функциональность обеспечивается модулем `sds-node-configurator` и включает в себя:

- Обнаружение блочных устройств на каждом узле и создание соответствующих ресурсов [BlockDevice](../../../../../reference/cr/blockdevices/).
  
{% alert level="warning" %}
Ручное создание и изменение ресурса [BlockDevice](../../../../../reference/cr/blockdevices/) запрещено.
{% endalert %}

- Автоматический поиск на узлах [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) с меткой `storage.deckhouse.io/enabled=true` (включая thin pool) и управление соответствующими ресурсами [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/). При обнаружении групп томов (Volume Group, VG) без ресурсов, контроллер создаёт ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) автоматически.

- Регулярное сканирование физических томов (Physical Volume, PV) на узлах, входящих в управляемые VG. При расширении нижестоящего блочного устройства контроллер выполняет `pvresize` для соответствующего физического тома и автоматически увеличивает размер всех логических томов, привязанных к этой группе.

{% alert level="warning" %}
Уменьшение размеров блочного устройства не поддерживается.
{% endalert %}

- Синхронизация состояния [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) с реальным состоянием узла: создание новых групп томов согласно [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/), расширение существующих при изменении `desiredCapacity` и удаление группы при удалении ресурса. Подробные примеры работы см. в разделе [Примеры работы с LVMVolumeGroup](./usage.html#работа-с-ресурсами-lvmvolumegroup).
