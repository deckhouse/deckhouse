---
title: "Диски"
permalink: ru/virtualization-platform/documentation/user/resource-managment/disks.html
lang: ru
---

Диски в виртуальных машинах необходимы для записи и хранения данных, обеспечивая полноценное функционирование приложений и операционных систем. Под "капотом" этих дисков используется хранилище, предоставляемое платформой.

В зависимости от свойств хранилища поведение дисков при создании и виртуальных машин в процессе эксплуатации может отличаться:

Свойство VolumeBindingMode:

`Immediate` - Диск создается сразу после создания ресурса (предполагается, что диск будет доступен для подключения к виртуальной машине на любом узле кластера).

![](images/vd-immediate.ru.png)

`WaitForFirstConsumer` - Диск создается только после того как будет подключен к виртуальной машине и будет создан на том узле, на котором будет запущена виртуальная машина.

![](images/vd-wffc.ru.png)

Режим доступа AccessMode:

- `ReadWriteOnce (RWO)` - доступ к диску предоставляется только одному экземпляру виртуальной машины. Живая миграция виртуальных машин с такими дисками невозможна.
- `ReadWriteMany (RWX)` - множественный доступ к диску. Живая миграция виртуальных машин с такими дисками возможна.

При создании диска контроллер самостоятельно определит наиболее оптимальные параметры поддерживаемые хранилищем.

Внимание: Создать диски из iso-образов - нельзя!

Чтобы узнать доступные варианты хранилищ на платформе, выполните следующую команду:

```bash
kubectl get storageclass

# NAME                          PROVISIONER                           RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
# i-linstor-thin-r1 (default)   replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
# i-linstor-thin-r2             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
# i-linstor-thin-r3             replicated.csi.storage.deckhouse.io   Delete          Immediate              true                   48d
# linstor-thin-r1               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
# linstor-thin-r2               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
# linstor-thin-r3               replicated.csi.storage.deckhouse.io   Delete          WaitForFirstConsumer   true                   48d
# nfs-4-1-wffc                  nfs.csi.k8s.io                        Delete          WaitForFirstConsumer   true                   30d
```

С полным описанием параметров конфигурации дисков можно ознакомиться по [ссылке](cr.html#virtualdisk).

### Создание пустого диска

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
    storageClassName: i-linstor-thin-r2
    size: 100Mi
EOF
```

После создания ресурс `VirtualDisk` может находиться в следующих состояниях (фазах):

- `Pending` - ожидание готовности всех зависимых ресурсов, требующихся для создания диска.
- `Provisioning` - идет процесс создания диска.
- `Resizing` - идет процесс изменения размера диска.
- `WaitForFirstConsumer` - диск ожидает создания виртуальной машины, которая будет его использовать.
- `Ready` - диск создан и готов для использования.
- `Failed` - произошла ошибка в процессе создания.
- `Terminating` - идет процесс удаления диска. Диск может "зависнуть" в данном состоянии если он еще подключен к виртуальной машине.

До тех пор пока диск не перешёл в фазу `Ready` содержимое всего блока `.spec` допускается изменять. При изменении процесс создании диска запустится заново.

Проверьте состояние диска после создание командой:

```bash
d8 k get vd blank-disk
# NAME       PHASE   CAPACITY   AGE
# blank-disk   Ready   100Mi      1m2s
```

### Создание диска из образа

Диск также можно создавать и заполнять данными из ранее созданных образов `ClusterVirtualImage` и `VirtualImage`.

При создании диска можно указать его желаемый размер, который должен быть равен или больше размера распакованного образа. Если размер не указан, то будет создан диск с размером, соответствующим исходному образу диска.

На примере ранее созданного проектного образа `VirtualImage`, рассмотрим команду позволяющую определить размер распакованного образа:

```bash
d8 k get cvi ubuntu-22.04 -o wide

# NAME           PHASE   CDROM   PROGRESS   STOREDSIZE   UNPACKEDSIZE   REGISTRY URL                                                                       AGE
# ubuntu-22.04   Ready   false   100%       285.9Mi      2.5Gi          dvcr.d8-virtualization.svc/cvi/ubuntu-22.04:eac95605-7e0b-4a32-bb50-cc7284fd89d0   122m
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
    storageClassName: i-linstor-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22.04
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
    storageClassName: i-linstor-thin-r2
  # Источник из которого создается диск.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualImage
      name: ubuntu-22.04
EOF
```

Проверьте состояние дисков после создания:

```bash
d8 k get vd

# NAME           PHASE   CAPACITY   AGE
# linux-vm-root    Ready   10Gi       7m52s
# linux-vm-root-2  Ready   2590Mi     7m15s
```

### Изменение размера диска

Размер дисков можно увеличивать, даже если они уже подключены к работающей виртуальной машине. Для этого отредактируйте поле `spec.persistentVolumeClaim.size`:

Проверим размер до изменения:

```bash
d8 k get vd linux-vm-root

# NAME          PHASE   CAPACITY   AGE
# linux-vm-root   Ready   10Gi       10m
```

Применим изменения:

```bash
kubectl patch vd linux-vm-root --type merge -p '{"spec":{"persistentVolumeClaim":{"size":"11Gi"}}}'
```

Проверим размер после изменения:

```bash
d8 k get vd linux-vm-root

# NAME          PHASE   CAPACITY   AGE
# linux-vm-root   Ready   11Gi       12m
```
