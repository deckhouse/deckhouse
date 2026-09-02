---
title: "Cloud provider — VMware vSphere: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развёрнутых в vSphere.

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

Источник настроек зависит от того, где размещён control plane кластера.
Если control plane работает на виртуальных машинах или bare metal, модуль использует собственные настройки, приведённые ниже.
Если control plane размещён в облаке, модуль использует ресурс [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration).

Количество узлов и параметры их заказа задаются в ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup) модуля `node-manager`.
Там же в параметре `cloudInstances.classReference` указывается инстанс-класс группы узлов.
Инстанс-классом для vSphere служит кастомный ресурс [VsphereInstanceClass](cr.html#vsphereinstanceclass), который описывает параметры самих виртуальных машин.

## Подключение к vCenter

Адрес vCenter и учётные данные задаются параметрами [`host`](#parameters-host), [`username`](#parameters-username) и [`password`](#parameters-password).

Модуль подключается к vCenter по TLS и проверяет его сертификат.
Если сертификат выпущен собственным или корпоративным центром сертификации, передайте цепочку сертификатов в параметре [`caBundle`](#parameters-cabundle).
Параметр [`insecure`](#parameters-insecure) со значением `true` полностью отключает проверку сертификата.
Задайте либо `caBundle`, либо `insecure: true`, поскольку одновременно эти параметры не принимаются.

## Хранилище

Модуль создаёт StorageClass для каждого Datastore и DatastoreCluster из используемых зон.
Если во vSphere настроены политики хранения SPBM, модуль дополнительно создаёт StorageClass для каждого сочетания Datastore и политики.

Ненужные StorageClass исключаются параметром [`exclude`](#parameters-storageclass-exclude), который принимает имена или регулярные выражения.
Выражение сопоставляется с именем Datastore, поэтому исключение убирает вместе с основным StorageClass и все StorageClass с политиками хранения для этого Datastore.

Чтобы задать StorageClass по умолчанию, используйте глобальный параметр [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass).
Параметр модуля [`default`](#parameters-storageclass-default) устарел.

### CSI

По умолчанию подсистема хранения использует диски CNS с возможностью изменения размера на лету.
Также поддерживается работа в legacy-режиме с дисками FCD, в котором изменение размера недоступно.
Режим выбирается параметром [`compatibilityFlag`](#parameters-storageclass-compatibilityflag).

### Увеличение размера PersistentVolumeClaim

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer CSI и API vSphere после увеличения размера PersistentVolumeClaim выполните следующие действия:

1. Выполните `d8 k cordon <NODE_NAME>` для узла, на котором работает под.
1. Удалите под, использующий PersistentVolumeClaim.
1. Дождитесь завершения операции. У PersistentVolumeClaim не должно остаться condition `Resizing`, при этом состояние `FileSystemResizePending` не является проблемой.
1. Выполните `d8 k uncordon <NODE_NAME>`.

{% include module-settings.liquid %}
