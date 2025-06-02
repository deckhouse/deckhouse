---
title: "FAQ"
permalink: ru/admin/configuration/storage/sds/node-configurator/faq.html
lang: ru
---

{% alert level="info" %}
Работоспособность гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](../../../../../supported_versions.html#linux). При использовании нестандартных ядер или дистрибутивов поведение может быть непредсказуемым.
{% endalert %}

## Причины отсутствия создания ресурсов BlockDevice в кластере

- Фильтрация устройств — чаще всего ресурсы [BlockDevice](../../../../../reference/cr/blockdevices/) не создаются, потому что имеющиеся устройства не проходят фильтры контроллера. Убедитесь, что устройства удовлетворяют [требованиям](./usage.html#критерии-отбора-устройства-контроллером).

## Причины отсутствия создания ресурсов LVMVolumeGroup в кластере

- Отсутствие [BlockDevice](../../../../../reference/cr/blockdevices/) — контроллер не создаст [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/), если в кластере нет ресурсов [BlockDevice](../../../../../reference/cr/blockdevices/), указанных в её спецификации.
- Отсутствие тега — если [BlockDevice](../../../../../reference/cr/blockdevices/) присутствуют, но [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) отсутствует, проверьте, что у соответствующей LVM-группы на узле задан тег `storage.deckhouse.io/enabled=true`.

## Причины, по которым после удаления ресурса LVMVolumeGroup ресурс и Volume Group остаются

Ситуация возможна в двух случаях:

1. В Volume Group имеются логические тома — контроллер не отвечает за удаление логических томов (Logical Volumes) на узле, поэтому, если в созданной посредством ресурса Volume Group имеются какие-либо логические тома, необходимо вручную удалить их. После этого и ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/), и сама Volume Group (включая физические тома) будут удалены автоматически.

1. На ресурсе есть аннотация `storage.deckhouse.io/deletion-protection` — данная аннотация защищает ресурс от удаления, а вместе с ним и связанную Volume Group. Уберите аннотацию командой:

   ```shell
   d8 k annotate lvg <имя-ресурса> storage.deckhouse.io/deletion-protection-
   ```

   После этого ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) и соответствующая Volume Group будут удалены автоматически.

## Причины неудачи создания Volume Group с помощью ресурса LVMVolumeGroup

Вероятнее всего ресурс не прошёл валидацию контроллера (в отличие от схемы Kubernetes). Причину можно узнать в поле `status.message` ресурса [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) или в логах контроллера.
Проверьте, что указанные [BlockDevice](../../../../../reference/cr/blockdevices/) соответствуют условиям:

- Поле `consumable` равно `true`;
- Для spec.type: Local все [BlockDevice](../../../../../reference/cr/blockdevices/) принадлежат одному узлу;
- Используются актуальные имена ресурсов [BlockDevice](../../../../../reference/cr/blockdevices/).

## Поведение ресурса LVMVolumeGroup при отключении одного из устройств в Volume Group

Ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) остаётся, пока существует одноимённая LVM‑группа. При недоступности устройства группа переходит в ошибочное состояние — это отражается в поле `status`.

После восстановления устройства группа возвращается в статус `Healthy`, а статус ресурса обновляется автоматически.

## Передача управления существующей на узле Volume Group контроллеру

Добавьте тег `storage.deckhouse.io/enabled=true` у нужной Volume Group:

```shell
vgchange myvg-0 --addtag storage.deckhouse.io/enabled=true
```

Контроллер создаст соответствующий ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) и возьмёт группу под управление.

## Отключение отслеживания LVM Volume Group контроллером

Удалите тег `storage.deckhouse.io/enabled=true`:

```shell
vgchange myvg-0 --deltag storage.deckhouse.io/enabled=true
```

Контроллер прекратит отслеживание и удалит связанный ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/).

## Причины автоматической установки тега storage.deckhouse.io/enabled=true на Volume Group

Контроллер добавляет тег при создании Volume Group через ресурс [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/).

При миграции с модуля `linstor` на `sds‑node‑configurator` и `sds‑replicated-volume` все теги `linstor-*` заменяются на `storage.deckhouse.io/enabled=true`, чтобы передать управление новой логике.

## Использование ресурса LVMVolumeGroupSet для создания LVMVolumeGroup

Ресурс [LVMVolumeGroupSet](../../../../../reference/cr/lvmvolumegroupset/) позволяет шаблонно создавать [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/) на узлах. Сейчас поддерживается стратегия PerNode — по одному ресурсу на каждый узел, удовлетворяющий селектору.

Пример спецификации [LVMVolumeGroupSet](../../../../../reference/cr/lvmvolumegroupset/):

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroupSet
metadata:
  name: my-lvm-volume-group-set
  labels:
    my-label: my-value
spec:
  strategy: PerNode
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/worker: ""
  lvmVolumeGroupTemplate:
    metadata:
      labels:
        my-label-for-lvg: my-value-for-lvg
    spec:
      type: Local
      blockDeviceSelector:
        matchLabels:
          status.blockdevice.storage.deckhouse.io/model: <model>
      actualVGNameOnTheNode: <имя-VG-на-узле>
```

## Метки, добавляемые контроллером к ресурсам BlockDevice

- `status.blockdevice.storage.deckhouse.io/type` — тип LVM;
- `status.blockdevice.storage.deckhouse.io/fstype` — тип файловой системы;
- `status.blockdevice.storage.deckhouse.io/pvuuid` — UUID физического тома (PV);
- `status.blockdevice.storage.deckhouse.io/vguuid` — UUID группы томов (VG);
- `status.blockdevice.storage.deckhouse.io/partuuid` — UUID раздела;
- `status.blockdevice.storage.deckhouse.io/lvmvolumegroupname` — имя ресурса [LVMVolumeGroup](../../../../../reference/cr/lvmvolumegroup/);
- `status.blockdevice.storage.deckhouse.io/actualvgnameonthenode` — имя LVM Volume Group на узле;
- `status.blockdevice.storage.deckhouse.io/wwn` — WWN (World Wide Name) устройства;
- `status.blockdevice.storage.deckhouse.io/serial` — серийный номер устройства;
- `status.blockdevice.storage.deckhouse.io/size` — размер устройства;
- `status.blockdevice.storage.deckhouse.io/model` — модель устройства;
- `status.blockdevice.storage.deckhouse.io/rota` — флаг ротационного устройства;
- `status.blockdevice.storage.deckhouse.io/hotplug` — возможность горячего подключения;
- `status.blockdevice.storage.deckhouse.io/machineid` — идентификатор машины, где установлено устройство.
