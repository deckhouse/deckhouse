---
title: "Модуль sds-node-configurator"
description: "Концепция и принцип работы модуля sds-node-configurator, Deckhouse Platform Certified Security Edition."
---
{% alert level="warning" %}
Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](https://deckhouse.ru/documentation/v1/supported_versions.html#linux).

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.
{% endalert %}

Модуль управляет `LVM` на узлах кластера через [пользовательские ресурсы Kubernetes](./cr.html), выполняя следующие операции:

  - Обнаружение блочных устройств и создание/обновление/удаление соответствующих им [ресурсов BlockDevice](./cr.html#blockdevice).

   > **Внимание!** Ручное создание и изменение ресурса `BlockDevice` запрещено.

  - Обнаружение на узлах `LVM Volume Group` с LVM тегом `storage.deckhouse.io/enabled=true` и `Thin-pool` на них, а также управление соответствующими [ресурсами LVMVolumeGroup](./cr.html#lvmvolumegroup). Модуль автоматически создает ресурс `LVMVolumeGroup`, если его еще не существует для обнаруженной `LVM Volume Group`.

  - Сканирование на узлах `LVM Physical Volumes`, которые входят в управляемые `LVM Volume Group`. В случае расширения размеров нижестоящих блочных устройств, соотвующие `LVM Physical Volumes` будут автоматически расширены (произойдёт `pvresize`).

  > **Внимание!** Уменьшение размеров блочного устройства не поддерживается.

  - Создание/расширение/удаление `LVM Volume Group` на узле в соответствии с пользовательскими изменениями в ресурсах `LVMVolumeGroup`. [Примеры использования](./usage.html#работа-с-ресурсами-lvmvolumegroup)
