---
title: "FAQ"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/sds/node-configurator/faq.html
lang: ru
---

{% alert level="info" %}
Работоспособность гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](/products/virtualization-platform/documentation/about/requirements.html). При использовании нестандартных ядер или дистрибутивов поведение может быть непредсказуемым.
{% endalert %}

## Причины, по которым в кластере не создаются ресурсы BlockDevice

Чаще всего ресурсы [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) не создаются, потому что имеющиеся устройства не проходят фильтры контроллера. Убедитесь, что устройства удовлетворяют [требованиям](./usage.html#критерии-отбора-устройства-контроллером).

## Причины, по которым в кластере не создаются ресурсы LVMVolumeGroup

- Отсутствие [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) — контроллер не создаст [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup), если в кластере нет ресурсов BlockDevice, указанных в её спецификации.
- Отсутствие тега — если ресурсы [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) присутствуют, но [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) отсутствует, проверьте, что у соответствующей LVM-группы на узле задан тег `storage.deckhouse.io/enabled=true`.

## Причины, по которым после удаления LVMVolumeGroup ресурс и Volume Group остаются

Ситуация возможна в двух случаях:

1. В Volume Group имеются логические тома — контроллер не отвечает за удаление логических томов (Logical Volumes) на узле, поэтому, если в созданной посредством ресурса Volume Group есть какие-либо логические тома, необходимо удалить их вручную. После этого и ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup), и сама Volume Group (включая физические тома) будут удалены автоматически.

1. Для ресурса установлена аннотация `storage.deckhouse.io/deletion-protection` — данная аннотация защищает ресурс от удаления, а вместе с ним и связанную Volume Group. Уберите аннотацию командой:

   ```shell
   d8 k annotate lvg <имя-ресурса> storage.deckhouse.io/deletion-protection-
   ```

   После этого ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) и соответствующая Volume Group будут удалены автоматически.

## Причины неуспешного создания Volume Group с помощью ресурса LVMVolumeGroup

Вероятнее всего ресурс не прошёл валидацию контроллера (в отличие от схемы Kubernetes). Причину можно узнать в поле `status.message` ресурса [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) или в логах контроллера.
Проверьте, что указанные [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) соответствуют условиям:

- поле `consumable` установлено в `true`;
- для `spec.type: Local` все [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice) принадлежат одному узлу;
- используются актуальные имена ресурсов [BlockDevice](/modules/sds-node-configurator/stable/cr.html#blockdevice).

## Поведение ресурса LVMVolumeGroup при отключении одного из устройств в Volume Group

Ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) остаётся, пока существует одноимённая LVM‑группа. При недоступности устройства группа переходит в ошибочное состояние — это отражается в поле `status`.

После восстановления устройства группа возвращается в статус `Healthy`, а статус ресурса обновляется автоматически.

## Передача управления существующей на узле Volume Group контроллеру

Добавьте тег `storage.deckhouse.io/enabled=true` нужной Volume Group:

```shell
vgchange myvg-0 --addtag storage.deckhouse.io/enabled=true
```

Контроллер создаст соответствующий ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) и возьмёт группу под управление.

## Отключение отслеживания LVM Volume Group контроллером

Удалите тег `storage.deckhouse.io/enabled=true`:

```shell
vgchange myvg-0 --deltag storage.deckhouse.io/enabled=true
```

Контроллер прекратит отслеживание и удалит связанный ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup).

## Причины автоматической установки тега storage.deckhouse.io/enabled=true для Volume Group

Контроллер добавляет тег при создании Volume Group через ресурс [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup).

При миграции с модуля `linstor` на `sds‑node‑configurator` и `sds‑replicated-volume` все теги `linstor-*` заменяются на `storage.deckhouse.io/enabled=true`, чтобы передать управление новой логике.

## Использование ресурса LVMVolumeGroupSet для создания LVMVolumeGroup

Ресурс [LVMVolumeGroupSet](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroupset) позволяет создавать [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) на узлах на основе шаблонов. Сейчас поддерживается стратегия PerNode — по одному ресурсу на каждый узел, удовлетворяющий селектору.

Пример спецификации [LVMVolumeGroupSet](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroupset):

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

## Лейблы, добавляемые контроллером к ресурсам BlockDevice

- `status.blockdevice.storage.deckhouse.io/type` — тип LVM;
- `status.blockdevice.storage.deckhouse.io/fstype` — тип файловой системы;
- `status.blockdevice.storage.deckhouse.io/pvuuid` — UUID физического тома (PV);
- `status.blockdevice.storage.deckhouse.io/vguuid` — UUID группы томов (VG);
- `status.blockdevice.storage.deckhouse.io/partuuid` — UUID раздела;
- `status.blockdevice.storage.deckhouse.io/lvmvolumegroupname` — имя ресурса [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup);
- `status.blockdevice.storage.deckhouse.io/actualvgnameonthenode` — имя LVM Volume Group на узле;
- `status.blockdevice.storage.deckhouse.io/wwn` — WWN (World Wide Name) устройства;
- `status.blockdevice.storage.deckhouse.io/serial` — серийный номер устройства;
- `status.blockdevice.storage.deckhouse.io/size` — размер устройства;
- `status.blockdevice.storage.deckhouse.io/model` — модель устройства;
- `status.blockdevice.storage.deckhouse.io/rota` — флаг ротационного устройства;
- `status.blockdevice.storage.deckhouse.io/hotplug` — возможность горячего подключения;
- `status.blockdevice.storage.deckhouse.io/machineid` — идентификатор машины, где установлено устройство.
