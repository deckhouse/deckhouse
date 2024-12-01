---
title: "Поддерживаемые хранилища"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/supported-storage.html
lang: ru
---

Для создания объектов StorageClass необходимо подключить одно или несколько хранилищ, которые обеспечат управление ресурсами `PersistentVolume`.

Все поддерживаемые системы можно разделить на две группы:

- программно определяемые хранилища SDS (Software-Defined Storage), расположенные на узлах платформы;
- внешние хранилища, которые могут быть размещены как на узлах платформы, так и за ее пределеами.

Программно определяемые хранилища:

- локальное блочное хранилище на основе LVM (Logical Volume Manager);
- реплицируемое блочное хранилище на основе DRBD (Distributed Replicated Block Device).

Внешне подключаемые хранилища:

- ceph-кластер;
- сетевая файловая система NFS (Network File System);
- система хранения данных TATLIN.UNIFIED (Yadro).

Созданные объекты StorageClass могут быть использованы для организации виртуальных дисков и образов.

## Как назначить StorageClass по умолчанию?

Чтобы назначить StorageClass по умолчанию, нужно указать желаемый класс хранения в глобальной конфигурации.
Пример команды:

```shell
# Укажите имя своего объекта StorageClass.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```

### StorageClass по умолчанию для виртуального образа

Альтернативно для виртуальных образов с типом хранения PersistentVolumeClaim можно установить отдельный StorageClass по умолчанию, который будет отличным от стандартного класса хранения на платформе.
При этом необходимо явно задать список классов хранения, которые пользователь сможет явно выбирать в конфигурации ресурса VirtualImage.

Для этого отредактируйте ModuleConfig `virtualization`:

```yaml
spec:
  settings:
    virtualImages:
      # Установите свой класс хранения по умолчанию.
      defaultStorageClassName: replicated-storage-class-r3
      # Установите свои классы хранения, разрешенные пользователю для создания виртуальных дисков.
      allowedStorageClassSelector:
        matchNames:
        - replicated-storage-class-r1
        - replicated-storage-class-r2
        - replicated-storage-class-r3
```

### StorageClass по умолчанию для виртуального диска

Альтернативно для виртуальных дисков можно установить отдельный StorageClass по умолчанию, который будет отличным от класса хранения по умолчанию на платформе.
При этом необходимо явно задать список классов хранения, которые пользователь сможет выбирать в конфигурации ресурса VirtualDisk.

Для этого отредактируйте ModuleConfig `virtualization`:

```yaml
spec:
  settings:
    virtualDisks:
      # Установить свой класс хранения по умолчанию:
      defaultStorageClassName: replicated-storage-class-r3
      # Установите свои классы хранения, разрешенные пользователю для создания виртуальных дисков:
      allowedStorageClassSelector:
        matchNames:
        - replicated-storage-class-r1
        - replicated-storage-class-r2
        - replicated-storage-class-r3
```

### StorageClass для реестра контейнеров

{% alert level="warning" %}
Изменение класса хранения по умолчанию для реестра контейнеров DVCR будет применено,
только если соответсвтующий PersistentVolumeClaim еще не был создан.
{% endalert %}

Для образов и дисков используется реестр контейнеров DVCR. Если реестр контейнеров DVCR использует PersistentVolumeClaim для хранения, то можно явно определить используемый StorageClass.

Для этого измените конфигурацию ModuleConfig `virtualization`:

```yaml
spec:
  settings:
    dvcr:
      storage:
        # Использовать PersistentVolumeClaim в качестве хранилища для реестра контейнеров.
        type: PersistentVolumeClaim
        persistentVolumeClaim:
          # Укажите имя своего StorageClass'a.
          storageClassName: replicated-storage-class-r3
```
