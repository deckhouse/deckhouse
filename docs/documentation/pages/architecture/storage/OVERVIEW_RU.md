---
title: Подсистема Storage
permalink: ru/architecture/storage/
lang: ru
search: storage, подсистема хранения, хранилище
description: Архитектура подсистемы Storage в Deckhouse Kubernetes Platform.
---

В данном подразделе описана архитектура подсистемы Storage (подсистемы хранения) Deckhouse Kubernetes Platform (DKP).

В подсистему Storage входят следующие модули:

* [`local-path-provisioner`](/modules/local-path-provisioner/) — предоставляет локальное хранилище на узлах Kubernetes с использованием томов `HostPath`. Создает ресурсы `StorageClass` для управления выделением локального хранилища;
* [`snapshot-controller`](/modules/snapshot-controller/) — включает поддержку снапшотов для совместимых CSI-драйверов в кластере Kubernetes;
* [`sds-local-volume`](/modules/sds-local-volume/) — управляет локальными блочными хранилищами на базе LVM, позволяет создавать `StorageClass` в Kubernetes с помощью кастомного ресурса [LocalStorageClass](https://deckhouse.ru/modules/sds-local-volume/cr.html#localstorageclass);
* [`sds-node-configurator`](/modules/sds-node-configurator/) — управляет блочными устройствами и LVM на узлах Kubernetes-кластера через [кастомные ресурсы Kubernetes](https://deckhouse.ru/modules/sds-node-configurator/stable/cr.html);
* [`sds-replicated-volume`](/modules/sds-replicated-volume/) — управляет реплицируемым блочным хранилищем на базе `DRBD`. В качестве control-plane/бэкенда используется `LINSTOR`;
* [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) — обеспечивает безопасные экспорт и импорт содержимого постоянных томов по протоколу HTTP;
* модули, предоставляющие реализацию CSI-драйвера для интеграции с различными типами хранилищ (программными и аппартными):

  * [`csi-ceph`](/modules/csi-ceph/);
  * [`csi-hpe`](/modules/csi-hpe/);
  * [`csi-huawei`](/modules/csi-huawei/);
  * [`csi-netapp`](/modules/csi-netapp/);
  * [`csi-nfs`](/modules/csi-nfs/);
  * [`csi-s3`](/modules/csi-s3/);
  * [`csi-scsi-generic`](/modules/csi-scsi-generic/);
  * [`csi-vsphere`](/modules/csi-vsphere/);
  * [`csi-csi-yadro-tatlin-unified`](/modules/csi-yadro-tatlin-unified/).

В подразделе на данный момент описан только [модуль local-path-provisioner](local-path-provisioner.html), материалы по остальным модулям подсистемы Storage будут добавляться по мере готовности.
