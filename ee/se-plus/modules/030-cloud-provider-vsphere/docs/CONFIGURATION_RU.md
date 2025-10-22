---
title: "Cloud provider — VMware vSphere: настройки"
force_searchable: true
---

Модуль автоматически включается для всех облачных кластеров, развернутых в vSphere.

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

Если control plane кластера размещен на виртуальных машинах или bare-metal-серверах, cloud-провайдер использует настройки модуля `cloud-provider-vsphere` в конфигурации Deckhouse (см. ниже). Иначе, если control plane кластера размещен в облаке, cloud-провайдер использует структуру [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) для настройки.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](../../modules/node-manager/cr.html#nodegroup) модуля `node-manager`, в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference` NodeGroup). Инстанс-класс для cloud-провайдера vSphere — это custom resource [`VsphereInstanceClass`](cr.html#vsphereinstanceclass), в котором указываются конкретные параметры самих машин.

## Storage

Модуль автоматически создает StorageClass для каждого Datastore и DatastoreCluster из зон (зоны).

Также он позволяет настроить имя StorageClass'а, который будет использоваться в кластере по умолчанию (параметр [default](#parameters-storageclass-default)) и отфильтровать ненужные StorageClass'ы (параметр [exclude](#parameters-storageclass-exclude)).

### CSI

Подсистема хранения по умолчанию использует CNS-диски с возможностью изменения их размера на лету. Но также поддерживается работа и в legacy-режиме с использованием FCD-дисков. Поведение настраивается параметром [compatibilityFlag](#parameters-storageclass-compatibilityflag).

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer CSI и vSphere API после увеличения размера PVC нужно сделать следующее:

1. На узле, где находится под, выполнить команду `d8 k cordon <имя_узла>`.
2. Удалить под.
3. Убедиться, что изменение размера прошло успешно. В объекте PVC *не будет* condition `Resizing`.
   > Состояние `FileSystemResizePending` не является проблемой.
4. На узле, где находится под, выполнить команду `d8 k uncordon <имя_узла>`.

{% include module-settings.liquid %}
