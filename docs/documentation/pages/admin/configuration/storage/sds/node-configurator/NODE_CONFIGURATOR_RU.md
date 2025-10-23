---
title: "Обзор"
permalink: ru/admin/configuration/storage/sds/node-configurator/about.html
description: "Настройка автоматического управления логическими томами в Deckhouse Kubernetes Platform. Автоматическое управление LVM, обнаружение блочных устройств и конфигурация хранилища на узлах кластера."
lang: ru
---

Deckhouse Kubernetes Platform предоставляет возможность автоматического управления логическими томами (Logical Volume Manager, LVM) на узлах кластера с помощью пользовательских ресурсов Kubernetes. Эта функциональность обеспечивается [модулем `sds-node-configurator`](/modules/sds-node-configurator/) и включает в себя:

- Обнаружение блочных устройств на каждом узле и создание соответствующих ресурсов [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice).
  
  > Ручное создание и изменение ресурса [BlockDevice](/modules/sds-node-configurator/cr.html#blockdevice) запрещено.

- Автоматический поиск на узлах [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) с лейблом `storage.deckhouse.io/enabled=true` (включая thin pool) и управление соответствующими ресурсами [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup). При обнаружении групп томов (Volume Group, VG) без ресурсов контроллер создаёт ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) автоматически.

- Регулярное сканирование физических томов (Physical Volume, PV) на узлах, входящих в управляемые VG. При расширении нижестоящего блочного устройства контроллер выполняет `pvresize` для соответствующего физического тома и автоматически увеличивает размер всех логических томов, привязанных к этой группе.

  > Уменьшение размеров блочного устройства не поддерживается.

- Синхронизация состояния [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) с реальным состоянием узла: создание новых групп томов согласно [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), расширение существующих при изменении `desiredCapacity` и удаление группы при удалении ресурса. Подробные примеры работы см. в разделе [Примеры работы с LVMVolumeGroup](./usage.html#работа-с-ресурсами-lvmvolumegroup).
