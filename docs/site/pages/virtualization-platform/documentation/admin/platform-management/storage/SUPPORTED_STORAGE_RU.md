---
title: "Поддерживаемые хранилища"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/supported-storage.html
lang: ru
---

Для создания StorageClass'ов необходимо подключить одно или несколько хранилищ, которые обеспечат управление PersistentVolume.

Все поддерживаемые системы можно разделить на две группы:

- программно определяемые хранилища SDS (Software-Defined Storage), расположенные на узлах платформы;
- внешние хранилаща, которые могут быть размещены как на узлах платформы, так и за ее пределеами.

Программно определяемые хранилища:

- локальное блочное хранилище на основе LVM (Logical Volume Manager);
- реплицируемое блочное хранилище на основе DRBD (Distributed Replicated Block Device).

Внешне подключаемые хранилища:

- ceph-кластер;
- сетевая файловая система NFS (Network File System);
- система хранения данных TATLIN.UNIFIED (Yadro).

Созданные StorageClass'ы могу быть использованы для создания виртуальных дисков и образов.

## Как назначить StorageClass по умолчанию?

Чтобы назначить StorageClass платформы по умолчанию, нужно указать желаемый класс хранения в глобальной конфигурации.
Пример команды для установки класс хранения по умолчанию:

```shell
# Укажите имя своего StorageClass'a.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
