---
title: "Диски"
permalink: ru/virtualization-platform/documentation/user/resource-management/disks.html
lang: ru
---

Диски в виртуальных машинах используются для записи и хранения данных, что необходимо для корректной работы приложений и операционных систем. Для этих целей в DVP можно использовать различные типы хранилищ.

В зависимости от выбранного типа хранилища, поведение дисков при создании виртуальных машин и в процессе эксплуатации может отличаться.

Поведение дисков при их создании зависит от параметра `VolumeBindingMode`, определяющего, когда именно создаётся диск и на каком узле:

`Immediate` — диск создается сразу после создания ресурса (предполагается, что диск будет доступен для подключения к виртуальной машине на любом узле кластера).

![Immediate](/images/virtualization-platform/vd-immediate.ru.png)

`WaitForFirstConsumer` — диск создается только после того, как будет подключен к виртуальной машине и будет создан на том узле, на котором будет запущена виртуальная машина.

![WaitForFirstConsumer](/images/virtualization-platform/vd-wffc.ru.png)

От параметра `AccessMode` зависит, как виртуальная машина сможет обращаться к диску — только она одна или несколько одновременно:

- `ReadWriteMany (RWX)` — множественный доступ к диску. Живая миграция виртуальных машин с такими дисками возможна.
- `ReadWriteOnce (RWO)` — доступ к диску предоставляется только одному экземпляру виртуальной машины. Живая миграция виртуальных машин с такими дисками поддерживается только для платных редакций DVP. Живая миграция доступна только если все диски подключены статически через `.spec.blockDeviceRefs`. Диски, подключенные динамически через `VirtualMachineBlockDeviceAttachments`, необходимо статически переподключить, указав их в `.spec.blockDeviceRefs`.

При создании диска контроллер самостоятельно определит наиболее оптимальные параметры поддерживаемые хранилищем.

{% alert level="warning" %}
Нельзя создавать диски из ISO-образов.
{% endalert %}

Чтобы узнать доступные варианты хранилищ, выполните следующую команду:

```bash
d8 k get storageclass
```

Пример вывода:

```console
NAME                                 PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
i-sds-replicated-thin-r1 (default)   replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
i-sds-replicated-thin-r2             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
i-sds-replicated-thin-r3             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
sds-replicated-thin-r1               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
sds-replicated-thin-r2               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
sds-replicated-thin-r3               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
nfs-4-1-wffc                         nfs.csi.k8s.io                        Delete          WaitForFirstConsumer   true                   30d
```

С полным описанием параметров конфигурации дисков можно ознакомиться [в документации ресурса VirtualDisk](/modules/virtualization/cr.html#virtualdisk).

Как узнать доступные варианты хранилищ веб-интерфейсе DVP:

- Перейдите на вкладку «Система», далее в раздел «Хранилище» → «Классы хранилищ».

## Создание пустого диска

Пустые диски обычно используются для установки на них ОС, либо для хранения каких-либо данных.

Создайте диск:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: blank-disk
spec:
  # Настройки параметров хранения диска.
  persistentVolumeClaim:
    # Подставьте ваше название StorageClass.
    storageClassName: i-sds-replicated-thin-r2
    size: 100Mi
EOF
```

После создания ресурс `VirtualDisk` может находиться в следующих состояниях (фазах):

- `Pending` — ожидание готовности всех зависимых ресурсов, требующихся для создания диска.
- `Provisioning` — идет процесс создания диска.
- `Resizing` — идет процесс изменения размера диска.
- `WaitForFirstConsumer` — диск ожидает создания виртуальной машины, которая будет его использовать.
- `WaitForUserUpload` — диск ожидает от пользователя загрузки образа (type: Upload).
- `Ready` — диск создан и готов для использования.
- `Failed` — произошла ошибка в процессе создания.
- `PVCLost` — системная ошибка, PVC с данными утерян.
- `Terminating` — идет процесс удаления диска. Диск может «зависнуть» в данном состоянии, если он еще подключен к виртуальной машине.

До тех пор, пока диск не перешёл в фазу `Ready` содержимое всего блока `.spec` допускается изменять. При изменении процесс создании диска запустится заново.

Диагностика проблем с ресурсом осуществляется путем анализа информации в блоке `.status.conditions`.

Если параметр `.spec.persistentVolumeClaim.storageClassName` не указан, то будет использован `StorageClass` по умолчанию на уровне кластера, либо для образов, если он указан в [настройках модуля](/products/virtualization-platform/documentation/admin/platform-management/virtualization/virtual-machine-classes.html).

Проверьте состояние диска после создания командой:

```bash
d8 k get vd blank-disk
```

Пример вывода:

```console
NAME       PHASE   CAPACITY   AGE
blank-disk   Ready   100Mi      1m2s
```

Как создать пустой диск в веб-интерфейсе (данный шаг можно пропустить и выполнить при создании ВМ):

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Диски ВМ».
- Нажмите «Создать диск».
- В открывшейся форме в поле «Имя диска» введите `blank-disk`.
- В поле «Размер» задайте размер с единицами измерений `100Mi`.
- В поле «Имя StorageClass» можно выбрать StorageClass или оставить выбранный по умолчанию.
- Нажмите кнопку «Создать».
- Статус диска отображается слева вверху, под именем диска.

## Создание диска из образа

Диск также можно создавать и заполнять данными из ранее созданных образов `ClusterVirtualImage` и `VirtualImage`.

При создании диска можно указать его желаемый размер, который должен быть равен или больше размера распакованного образа. Если размер не указан, то будет создан диск с размером, соответствующим исходному образу диска.

На примере ранее созданного проектного образа `VirtualImage`, рассмотрим команду, позволяющую определить размер распакованного образа:

```bash
d8 k get cvi ubuntu-22-04 -o wide
```

Пример вывода:

```console
NAME           PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                                                       AGE
ubuntu-22-04   Ready   false   100%       285.9Mi      2.5Gi          dvcr.d8-virtualization.svc/cvi/ubuntu-22-04:eac95605-7e0b-4a32-bb50-cc7284fd89d0   122m
```

Искомый размер указан в колонке **UNPACKEDSIZE** и равен 2.5Gi.

Создадим диск из этого образа:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Настройки параметров хранения диска.
  persistentVolumeClaim:
    # Укажем размер больше чем значение распакованного образа.
    size: 10Gi
    # Подставьте ваше название StorageClass.
    storageClassName: i-sds-replicated-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22-04
EOF
```

А теперь создайте диск, без явного указания размера:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root-2
spec:
  # Настройки параметров хранения диска.
  persistentVolumeClaim:
    # Подставьте ваше название StorageClass.
    storageClassName: i-sds-replicated-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22-04
EOF
```

Проверьте состояние дисков после создания:

```bash
d8 k get vd
```

Пример вывода:

```console
NAME           PHASE   CAPACITY   AGE
linux-vm-root    Ready   10Gi       7m52s
linux-vm-root-2  Ready   2590Mi     7m15s
```

Как создать диск из образа в веб-интерфейсе (данный шаг можно пропустить и выполнить при создании ВМ):

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Диски ВМ».
- Нажмите «Создать диск».
- В открывшейся форме в поле «Имя диска» введите `linux-vm-root`.
- В поле «Источник» убедитесь, что установлен чек-бокс «Проектные».
- Из выпадающего списка выберите интересующий Вас образ.
- В поле «Размер» можете изменить размер на больший или оставить выбранный по умолчанию.
- В поле «Имя StorageClass» можно выбрать StorageClass или оставить выбранный по умолчанию.
- Нажмите кнопку «Создать».
- Статус диска отображается слева вверху, под именем диска.

## Изменение размера диска

Размер дисков можно увеличивать, даже если они уже подключены к работающей виртуальной машине. Для этого отредактируйте поле `spec.persistentVolumeClaim.size`:

Проверьте размер до изменения:

```bash
d8 k get vd linux-vm-root
```

Пример вывода:

```console
NAME          PHASE   CAPACITY   AGE
linux-vm-root   Ready   10Gi       10m
```

Примените изменения:

```bash
d8 k patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'
```

Проверьте размер после изменения:

```bash
d8 k get vd linux-vm-root
```

Пример вывода:

```console
NAME          PHASE   CAPACITY   AGE
linux-vm-root   Ready   11Gi       12m
```

Как изменить размер диска в веб-интерфейсе:

Способ №1:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Диски ВМ».
- Выберите нужный диск и нажмите на символ карандаша в колонке «Размер».
- Во всплывающем окне можете изменить размер на больший.
- Нажмите на кнопку «Применить».
- Статус диска отображается в колонке «Статус».

Способ №2:

- Перейдите на вкладку «Проекты» и выберите нужный проект.
- Перейдите в раздел «Виртуализация» → «Диски ВМ».
- Выберите нужный диск и нажмите на его имя.
- В открывшейся форме на вкладке «Конфигурация» в поле «Размер» можете изменить размер на больший.
- Нажмите на появившуюся кнопку «Сохранить».
- Статус диска отображается слева вверху, под его именем.

### Изменение класса хранения диска

В платных редакциях DVP можно изменить класс хранения для существующих дисков. Сейчас это поддерживается только для работающих ВМ (`Phase` должна быть `Running`).

{% alert level="warning" %}
Миграция класса хранения поддерживается только для дисков, статически подключенных через параметр `.spec.blockDeviceRefs` в конфигурации виртуальной машины.

Для миграции класса хранения дисков, подключенных через `VirtualMachineBlockDeviceAttachments`, необходимо переподключить их статически, указав имена дисков в `.spec.blockDeviceRefs`.
{% endalert %}

Пример:

```bash
d8 k patch vd disk --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
```

После изменения конфигурации диска запустится живая миграция ВМ, в процессе которой диск ВМ будет мигрирован на новое хранилище.

Если к виртуальной машине подключены несколько дисков и требуется изменить класс хранения для нескольких дисков, эту операцию необходимо выполнить последовательно:

```bash
d8 k patch vd disk1 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
d8 k patch vd disk2 --type=merge --patch '{"spec":{"persistentVolumeClaim":{"storageClassName":"new-storage-class-name"}}}'
```

При неуспешной миграции повторные попытки выполняются с увеличивающимися задержками (алгоритм экспоненциального backoff). Максимальная задержка — 300 секунд (5 минут). Задержки: 5 секунд (1-я попытка), 10 секунд (2-я), далее каждая задержка удваивается, достигая 300 секунд (7-я и последующие попытки). Первая попытка выполняется без задержки.

Для отмены миграции пользователь должен вернуть класс хранения в спецификации на исходный.
