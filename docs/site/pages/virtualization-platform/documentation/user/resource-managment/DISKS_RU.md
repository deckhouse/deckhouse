---
title: "Диски"
permalink: ru/virtualization-platform/documentation/user/resource-management/disks.html
lang: ru
---

Диски в виртуальных машинах (ресурсы [VirtualDisk](../../../reference/cr/virtualdisk.html)) необходимы для записи и хранения данных. Они обеспечивают полноценное функционирование приложений и операционных систем. Структура этих дисков включает хранилище, предоставляемое платформой.

В зависимости от свойств хранилища, диски при создании и виртуальные машины во время эксплуатации могут проявлять разное поведение.

Свойства `VolumeBindingMode`:

`Immediate` — Диск создается сразу после создания ресурса (предполагается, что диск будет доступен для подключения к виртуальной машине на любом узле кластера).

![Immediate](/images/virtualization-platform/vd-immediate.ru.png)

`WaitForFirstConsumer` — Диск создается только после того как будет подключен к виртуальной машине и будет создан на том узле, на котором будет запущена виртуальная машина.

![WaitForFirstConsumer](/images/virtualization-platform/vd-wffc.ru.png)

Режим доступа AccessMode:

- `ReadWriteOnce (RWO)` — доступ к диску предоставляется только одному экземпляру виртуальной машины. Миграция виртуальных машин в реальном времени с такими дисками невозможна.
- `ReadWriteMany (RWX)` — множественный доступ к диску. Миграция виртуальных машин в реальном времени с такими дисками возможна.

При создании диска контроллер самостоятельно определит наиболее оптимальные параметры поддерживаемые хранилищем.

{% alert level="warning" %}
Создать диски из ISO-образов — нельзя!
{% endalert %}

Чтобы узнать доступные варианты хранилищ на платформе, выполните следующую команду:

```bash
kubectl get storageclass
```

Пример вывода команды:

```console
# NAME                          PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# i-linstor-thin-r1 (default)   replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
# i-linstor-thin-r2             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
# i-linstor-thin-r3             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
# linstor-thin-r1               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
# linstor-thin-r2               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
# linstor-thin-r3               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
# nfs-4-1-wffc                  nfs.csi.k8s.io                        Delete          WaitForFirstConsumer   true                   30d
```

### Создание пустого диска

Пустые диски обычно используются для установки на них ОС, либо для хранения каких-либо данных.

Для создания диска:

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
    storageClassName: i-linstor-thin-r2
    size: 100Mi
EOF
```

После создания ресурс [VirtualDisk](../../../reference/cr/virtualdisk.html) может находиться в следующих состояниях:

- `Pending` — ожидание готовности всех зависимых ресурсов, требующихся для создания диска.
- `Provisioning` — идет процесс создания диска.
- `Resizing` — идет процесс изменения размера диска.
- `WaitForFirstConsumer` — диск ожидает создания виртуальной машины, которая будет его использовать.
- `Ready` — диск создан и готов для использования.
- `Failed` — произошла ошибка в процессе создания.
- `Terminating` — идет процесс удаления диска. Процесс может «зависнуть» в этом состоянии если он еще подключен к виртуальной машине.

До тех пор, пока диск не перешёл в фазу `Ready`, содержимое всего блока `.spec` допускается изменять. При изменении процесс создании диска запустится заново.

Проверьте состояние диска после создания:

```bash
d8 k get vd blank-disk
```

Пример вывода:

```console
# NAME       PHASE   CAPACITY   AGE
# blank-disk   Ready   100Mi      1m2s
```

### Создание диска из образа

Диск можно создавать и заполнять данными из ранее созданных образов [ClusterVirtualImage](../../../reference/cr/clustervirtualimage.html) и [VirtualImage](../../../reference/cr/virtualimage.html).

При создании диска можно указать его желаемый размер, который должен быть равен или больше размера распакованного образа. Если размер не указан, то будет создан диск с размером, соответствующим исходному размеру образа диска.

На примере ранее созданного проектного образа [VirtualImage](../../../reference/cr/virtualimage.html), рассмотрим команду позволяющую определить размер распакованного образа:

```bash
d8 k get cvi ubuntu-22.04 -o wide
```

Пример вывода:

```console
# NAME           PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                                                       AGE
# ubuntu-22.04   Ready   false   100%       285.9Mi      2.5Gi          dvcr.d8-virtualization.svc/cvi/ubuntu-22.04:eac95605-7e0b-4a32-bb50-cc7284fd89d0   122m
```

Искомый размер указан в колонке UNPACKEDSIZE и равен 2.5Gi.

Создайте диск из этого образа:

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
    storageClassName: i-linstor-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22.04
EOF
```

А теперь создайте диск без указания размера:

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
    storageClassName: i-linstor-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22.04
EOF
```

Проверьте состояние дисков после создания командой:

```bash
d8 k get vd
```

Пример вывода:

```console
# NAME           PHASE   CAPACITY   AGE
# linux-vm-root    Ready   10Gi       7m52s
# linux-vm-root-2  Ready   2590Mi     7m15s
```

### Изменение размера диска

Размер дисков можно увеличивать, даже если они уже подключены к работающей виртуальной машине. Изменения вносятся в поле `spec.persistentVolumeClaim.size`:

Проверьте размер до изменения командой:

```bash
d8 k get vd linux-vm-root
```

Пример вывода:

```console
# NAME          PHASE   CAPACITY   AGE
# linux-vm-root   Ready   10Gi       10m
```

Примените изменения:

```bash
kubectl patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'
```

Проверьте размер после изменения:

```bash
d8 k get vd linux-vm-root
```

Пример вывода:

```console
# NAME          PHASE   CAPACITY   AGE
# linux-vm-root   Ready   11Gi       12m
```
