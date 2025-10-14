---
title: " Модуль sds-node-configurator: FAQ"
description: "Модуль sds-node-configurator, Deckhouse Platform Certified Security Edition. Частые вопросы и ответы."
---
{% alert level="warning" %}
Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с поддерживаемыми дистрибутивами.

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.
{% endalert %}

## Почему в кластере не создаются ресурсы `BlockDevice` и `LVMVolumeGroup`?

* В большинстве случаев `BlockDevice`-ресурсы могут не создаваться по причине того, что существующие девайсы не проходят фильтрацию на стороне контроллера. Пожалуйста, убедитесь, что ваши девайсы соответствуют указанным [требованиям](./usage.html#требования-контроллера-к-девайсу).

* `LVMVolumeGroup`-ресурсы могут не создаваться по причине отсутствия в кластере `BlockDevice`-ресурсов, так как их имена используются в спецификации `LVMVolumeGroup`.

* В том случае, если `BlockDevice`-ресурсы существуют, а `LVMVolumeGroup`-ресурсы отсутствуют, пожалуйста, убедитесь, что у существующих `LVM Volume Group` на узле имеется специальный тег `storage.deckhouse.io/enabled=true`.

## Я выполнил команду на удаление ресурса `LVMVolumeGroup`, но и ресурс, и `Volume Group` осталась. Почему так?

Такая ситуация возможна в двух случаях: 

1. В `Volume Group` имеются `LV`. 
Контроллер не берет ответственность за удаление LV с узла, поэтому, если в созданной с помощью ресурса `Volume Group` имеются какие-либо логические тома, Вам необходимо вручную удалить их на узле. После этого и ресурс, и `Volume Group` (вместе с `PV`) будут удалены автоматически.

2. На ресурсе имеется аннотация `storage.deckhouse.io/deletion-protection`.
Данная аннотация защищает удаление ресурса и, как следствие, созданной им `Volume Group`. Вам необходимо самостоятельно убрать аннотацию командой 
```shell
kubectl annotate lvg %lvg-name% storage.deckhouse.io/deletion-protection-
```

После выполнения данной команды и ресурс, и `Volume Group` будут удалены автоматически.

## Я пытаюсь создать `Volume Group`, используя ресурс `LVMVolumeGroup`, но у меня ничего не получается. Почему?

Скорее всего, ваш ресурс не проходит валидацию со стороны контроллера (при этом, валидация со стороны Kubernetes прошла успешно).
С конкретной причиной неработоспособности вы можете ознакомиться в самом ресурсе в поле `status.message` либо обратиться
к логам контроллера.

Как правило, проблема кроется в некорректно указанных ресурсах `BlockDevice`. Пожалуйста, убедитесь, что выбранные
ресурсы удовлетворяют следующим требованиям:
- Поле `Consumable` имеет значение `true`.
- Для `Volume Group` типа `Local` указанные `BlockDevice` принадлежат одному узлу.<!-- > - Для `Volume Group` типа `Shared` указан единственный ресурс `BlockDevice`. -->
- Указаны актуальные имена ресурсов `BlockDevice`.

С полным списком ожидаемых значений вы можете ознакомиться с помощью [CR-референса](./cr.html) `LVMVolumeGroup`-ресурса.

## Что произойдет, если я отключу один из девайсов в `Volume Group`? Соответствующий ресурс `LVMVolumeGroup` удалится?

Ресурс `LVMVolumeGroup` будет существовать до тех пор, пока существует соответствующая `Volume Group`. До тех пор, пока
существует хоть один девайс, `Volume Group` будет существовать, но в «нездоровом» состоянии.
Эти проблемы будут отображены в `status` ресурса.

После восстановления отключенного девайса на узле, `LVM Volume Group` восстановит свою работоспособность и соответствующий ресурс `LVMVolumeGroup` также отобразит актуальное состояние.

## Как передать контроллеру управление существующей на узле `LVM Volume Group`?

Достаточно добавить LVM-тег `storage.deckhouse.io/enabled=true` на `LVM Volume Group` на узле: 

```shell
vgchange myvg-0 --addtag storage.deckhouse.io/enabled=true
```

## Я хочу, чтобы контроллер перестал следить за `LVM Volume Group` на узле. Как мне это сделать?

Достаточно удалить LVM-тег `storage.deckhouse.io/enabled=true` у нужной `LVM Volume Group` на узле:

```shell
vgchange myvg-0 --deltag storage.deckhouse.io/enabled=true
```

После этого контроллер перестанет отслеживать выбранную `Volume Group` и самостоятельно удалит связанный с ней ресурс `LVMVolumeGroup`.

## Я не вешал LVM-тег `storage.deckhouse.io/enabled=true` на `Volume Group`, но он появился. Как это возможно?

Это возможно в случае, если вы создавали `LVM Volume Group` через ресурс `LVMVolumeGroup` (в таком случае контроллер автоматически вешает данный LVM-тег на созданную `LVM Volume Group`). Либо на данной `Volume Group` или ее `Thin-pool` был LVM-тег модуля `linstor` — `linstor-*`.

При миграции с встроенного модуля `linstor` на модули `sds-node-configurator` и `sds-replicated-volume` автоматически происходит изменение LVM-тегов `linstor-*` на LVM-тег `storage.deckhouse.io/enabled=true` в `Volume Group`. Таким образом, управление этими `Volume Group` передается модулю `sds-node-configurator`.

## Как использовать ресурс `LVMVolumeGroupSet` для создания `LVMVolumeGroup`?

Для создания `LVMVolumeGroup` с помощью `LVMVolumeGroupSet` необходимо указать в спецификации `LVMVolumeGroupSet` селекторы для узлов и шаблон для создаваемых ресурсов `LVMVolumeGroup`. На данный момент поддерживается только стратегия `PerNode`, при которой контроллер создаст по одному ресурсу `LVMVolumeGroup` из шаблона для каждого узла, удовлетворяющего селектору.

Пример спецификации `LVMVolumeGroupSet`:

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
      actualVGNameOnTheNode: <actual-vg-name-on-the-node>


```

## Как использовать ресурс `LVMVolumeGroupSet` для создания `LVMVolumeGroup`?

Для создания `LVMVolumeGroup` с помощью `LVMVolumeGroupSet` необходимо указать в спецификации `LVMVolumeGroupSet` селекторы для узлов и шаблон для создаваемых ресурсов `LVMVolumeGroup`. На данный момент поддерживается только стратегия `PerNode`, при которой контроллер создаст по одному ресурсу `LVMVolumeGroup` из шаблона для каждого узла, удовлетворяющего селектору.

Пример спецификации `LVMVolumeGroupSet`:

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
      actualVGNameOnTheNode: <actual-vg-name-on-the-node>


```

## Какие лейблы добавляются контроллером на ресурсы BlockDevices

* status.blockdevice.storage.deckhouse.io/type - тип LVM

* status.blockdevice.storage.deckhouse.io/fstype - тип файловой системы

* status.blockdevice.storage.deckhouse.io/pvuuid - UUID PV

* status.blockdevice.storage.deckhouse.io/vguuid - UUID VG

* status.blockdevice.storage.deckhouse.io/partuuid - UUID раздела

* status.blockdevice.storage.deckhouse.io/lvmvolumegroupname - имя этого ресурса

* status.blockdevice.storage.deckhouse.io/actualvgnameonthenode - название VolumeGroup на ноде

* status.blockdevice.storage.deckhouse.io/wwn - идентификатор WWN (World Wide Name) для устройства

* status.blockdevice.storage.deckhouse.io/serial - серийный номер устройства

* status.blockdevice.storage.deckhouse.io/size - раздел

* status.blockdevice.storage.deckhouse.io/model - модель устройства

* status.blockdevice.storage.deckhouse.io/rota - является ли ротационным  устройством

* status.blockdevice.storage.deckhouse.io/hotplug - возможность hot подключения

* status.blockdevice.storage.deckhouse.io/machineid - ID сервера, на котором установлено блочное устройство
