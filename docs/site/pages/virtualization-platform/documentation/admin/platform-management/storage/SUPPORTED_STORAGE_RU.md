---
title: "Поддерживаемые хранилища"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/supported-storage.html
lang: ru
---

Для создания StorageClass’ов необходимо подключить одно или несколько хранилищ, которые обеспечат управление PersistentVolume’ами.

Все поддерживаемые системы можно разделить на две группы:
- программно определяемые хранилища SDS (Software-Defined Storage), расположенные на узлах платформы; 
- внешние хранилаща, которые могут быть размещены как на узлах платформы, так и за ее пределеами.

Программно определяемые хранилища:
- локальное блочное хранилище на основе LVM (Logical Volume Manager);
- реплицируемое блочное хранилище на основе DRBD (Distributed Replicated Block Device).

Внешне подключаемые хранилища:
- Ceph-кластер;
- сетевая файловая система NFS (Network File System);
- система хранения данных TATLIN.UNIFIED (Yadro).

Созданные StorageClass’ы могу быть использованы для создания виртуальных дисков и образов.

## Как назначить StorageClass по умолчанию?

