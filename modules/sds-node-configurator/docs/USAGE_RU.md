---
title: "Модуль sds-node-configurator: примеры использования"
description: Использование и примеры работы контроллера sds-node-configurator. Deckhouse Kubernetes Platform.
---

{{< alert level="warning" >}}
Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](https://deckhouse.ru/documentation/v1/supported_versions.html#linux).

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.
{{< /alert >}}

Контроллер работает с двумя типами ресурсов:
* `BlockDevice`;
* `LVMVolumeGroup`.

## Работа с ресурсами `BlockDevice`

### Создание ресурса `BlockDevice`

Контроллер регулярно сканирует существующие девайсы на узле, и в случае, если девайс удовлетворяет всем необходимым
условиям со стороны контроллера, создается `custom resource` (CR) `BlockDevice` с уникальным именем, в котором отображена
полная и необходимая информация о соответствующем девайсе.

#### Требования контроллера к девайсу

* Не является drbd-устройством.
* Не является псевдодевайсом (то есть не loop device).
* Не является `Logical Volume`.
* Файловая система отсутствует или соответствует `LVM2_MEMBER`.
* У блок-девайса отсутствуют партиции.
* Размер блок-девайса больше 1 Gi.
* Если девайс — виртуальный диск, у него должен быть серийный номер.

Информацию из полученного ресурса контроллер будет использовать для своей дальнейшей работы с ресурсами `LVMVolumeGroup`.

### Обновление ресурса `BlockDevice`

Контроллер самостоятельно обновляет информацию в ресурсе, если состояние указанного в нем блок-девайса поменялось на узле.

### Удаление ресурса `BlockDevice`

Контроллер автоматически удалит ресурс, если указанный в нем блок-девайс стал недоступен. Удаление произойдёт только в следующих случаях:
* если ресурс был в статусе Consumable;
* если блок-девайс принадлежит `Volume Group`, у которой нет LVM-тега `storage.deckhouse.io/enabled=true` (эта `Volume Group` не управляется нашим контроллером).

> Контроллер выполняет вышеперечисленные виды работ автоматически и не требует вмешательства со стороны пользователя.

> В случае ручного удаления ресурса, он будет пересоздан контроллером.

## Работа с ресурсами `LVMVolumeGroup`

Ресурсы `BlockDevice` необходимы для создания и обновления ресурсов `LVMVolumeGroup`. 
На данный момент поддерживаются только локальные `Volume Group`.
`LVMVolumeGroup`-ресурсы предназначены для взаимодействия с `LVM Volume Group` на узлах и отображения актуальной информации об их состоянии.

### Создание ресурса `LVMVolumeGroup`

Ресурс `LVMVolumeGroup` может быть создан 2 способами:
* Автоматическое создание:
  * Контроллер автоматически сканирует информацию о существующих `LVM Volume Group` на узлах и создает ресурс в случае, если у `LVM Volume Group` имеется LVM-тег `storage.deckhouse.io/enabled=true` и соответствующий ей Kubernetes-ресурс отсутствует.
  * В этом случае контроллер самостоятельно заполнит все поля `Spec`-секции ресурса, кроме поля `thinPools`. Пользователю необходимо вручную добавить в `Spec` ресурса информацию о `Thin-pool`, имеющимся на узле, в случае, если он хочет, чтобы данный `Thin-pool` попал под управление контроллера.
* Пользовательское создание:
  * Пользователь вручную создает ресурс, заполняя только поля `metadata.name` и `spec`, в котором указывает желаемое состояние новой `Volume Group`.
  * Конфигурация, указанная пользователем, пройдет специальную валидацию на корректность.
  * После успешного прохождения валидации контроллер использует указанную конфигурацию, чтобы по ней создать указанную `LVM Volume Group` на узле и обновить пользовательский ресурс актуальной информацией о состоянии созданной `LVM Volume Group`.
  * Пример ресурса для создания локальной `LVM Volume Group` из нескольких `BlockDevice`:

    ```yaml
    apiVersion: storage.deckhouse.io/v1alpha1
    kind: LVMVolumeGroup
    metadata:
      name: "vg-0-on-node-0"
    spec:
      type: Local
      local:
        nodeName: "node-0"
      blockDeviceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
          - dev-07ad52cef2348996b72db262011f1b5f896bb68f
          - dev-e90e8915902bd6c371e59f89254c0fd644126da7
      actualVGNameOnTheNode: "vg-0"
    ```

    ```yaml
    apiVersion: storage.deckhouse.io/v1alpha1
    kind: LVMVolumeGroup
    metadata:
      name: "vg-0-on-node-0"
    spec:
      type: Local
      local:
        nodeName: "node-0"
      blockDeviceSelector:
        matchLabels:
          kubernetes.io/hostname: node-0
      actualVGNameOnTheNode: "vg-0"
    ```

  * Пример ресурса для создания локальной `LVM Volume Group` и `Thin-pool` на ней из нескольких `BlockDevice`:

    ```yaml
    apiVersion: storage.deckhouse.io/v1alpha1
    kind: LVMVolumeGroup
    metadata:
      name: "vg-0-on-node-0"
    spec:
      type: Local
      local:
        nodeName: "node-0"
      blockDeviceSelector:
        matchExpressions:
        - key: kubernetes.io/metadata.name
          operator: In
          values:
          - dev-07ad52cef2348996b72db262011f1b5f896bb68f
          - dev-e90e8915902bd6c371e59f89254c0fd644126da7
      actualVGNameOnTheNode: "vg-0"
      thinPools:
      - name: thin-1
        size: 250Gi
    ```

    ```yaml
    apiVersion: storage.deckhouse.io/v1alpha1
    kind: LVMVolumeGroup
    metadata:
      name: "vg-0-on-node-0"
    spec:
      type: Local
      local:
        nodeName: "node-0"
      blockDeviceSelector:
        matchLabels:
          kubernetes.io/hostname: node-0
      actualVGNameOnTheNode: "vg-0"
      thinPools:
      - name: thin-1
        size: 250Gi
    ```

  > Вы можете указать любые удобные для Вас селекторы для ресурсов `BlockDevice`. Так, например, Вы можете выбрать все девайсы на этом узле (используя, например, `matchLabels`), либо выбрать часть, дополнительно указав их имена (или иные другие параметры).
  > Обратите внимание, что поле `spec.local` является обязательным для типа `Local`. В случае расхождения имени в поле `spec.local.nodeName` и селекторах создание LVMVolumeGroup выполнено не будет.
  
  > **Внимание!** Все выбранные блок-девайсы должны принадлежать одному узлу для `LVMVolumeGroup` с типом 'Local'. 

### Обновление ресурса `LVMVolumeGroup`
Вы можете изменить желаемое состояние `VolumeGroup` или `thin pool` на узлах с помощью изменения поля `spec` соответствующего ресурса `LVMVolumeGroup`. Контроллер автоматически провалидирует новые данные и, в случае их валидного состояния, внесет необходимые изменения в сущности на узле.

Контроллер в автоматическом режиме обновляет поле `status` ресурса `LVMVolumeGroup`, отображая актуальные данные о соответствующей `LVM Volume Group` на узле.
Пользователю **не рекомендуется** собственноручно вносить изменения в поле `status`.

> Контроллер не обновляет поле `spec`, так как указанное поле отображает желаемое состояние `LVM Volume Group`. Пользователь может вносить изменения в поле `spec`, чтобы изменить состояние указанной в ресурсе `LVM Volume Group` на узле.

### Удаление ресурса `LVMVolumeGroup`

Контроллер автоматически удалит ресурс, если указанная в нем `Volume Group` стала недоступна по той или иной причине (например на узле были отключены все блочные устройства, из которых состояла `Volume Group`).

Пользователь может удалить `LVM Volume Group` с узла и связанные с ним `LVM Physical Volume`, выполнив команду на удаление ресурса `LVMVolumeGroup`.

```shell
kubectl delete lvg %lvg-name%
```

### Вывод ресурса `BlockDevice` из `LVMVolumeGroup` ресурса
Для того чтобы вывести `BlockDevice` ресурс из `LVMVolumeGroup` ресурса, необходимо либо изменить поле `spec.blockDeviceSelector` `LVMVolumeGroup` ресурса (добавить другие селекторы), либо изменить соответствующие лейблы у `BlockDevice` ресурса, чтобы они больше не попадали под селекторы `LVMVolumeGroup`. После этого вам необходимо вручную выполнить команды `pvmove`, `vgreduce`, и `pvremove` на узле.

> **Внимание!** Если удаляемый ресурс `LVMVolumeGroup` содержит `Logical Volume` (даже если это только `Thin-pool`, который указан в `spec`) пользователю необходимо самостоятельно удалить все `Logical Volume`, которые содержит удаляемая `Volume Group`. В противном случае ни ресурс, ни `Volume Group` удалены не будут.

> Пользователь может запретить удаление `LVMVolumeGroup` ресурса, повесив на ресурс специальную аннотацию `storage.deckhouse.io/deletion-protection`. При наличии данной аннотации контроллер не будет удалять ни ресурс, ни соответствующую `Volume Group` до тех пор, пока аннотация не будет снята с ресурса.
