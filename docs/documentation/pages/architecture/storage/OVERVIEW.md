---
title: Storage subsystem
permalink: en/architecture/storage/
search: storage, storage subsystem
description: Architecture of the Storage subsystem in Deckhouse Kubernetes Platform.
---

This subsection describes the architecture of the Storage subsystem of Deckhouse Kubernetes Platform (DKP).

The Storage subsystem includes the following modules:

* [`local-path-provisioner`](/modules/local-path-provisioner/): Provides the local storage on Kubernetse nodes using HostPath volumes. Creates StorageClass resources to manage the allocation of local storage.
* [`snapshot-controller`](/modules/snapshot-controller/): Enables snapshot support for compatible CSI-drivers in the Kubernetes cluster.
* [`sds-local-volume`](/modules/sds-local-volume/): Manages the local block storage based on LVM. It enables creating StorageClasses in Kubernetes using the [LocalStorageClass](https://deckhouse.ru/modules/sds-local-volume/cr.html#localstorageclass) resource.
* [`sds-node-configurator`](/modules/sds-node-configurator/): Manages block devices and LVM on Kubernetes cluster nodes through [Kubernetes custom resources](https://deckhouse.ru/modules/sds-node-configurator/stable/cr.html).
* [`sds-replicated-volume`](/modules/sds-replicated-volume/): Manages replicated block storage based on `DRBD`. Currently, `LINSTOR` is used as a control-plane/backend.
* [`storage-volume-data-manager`](/modules/storage-volume-data-manager/): Provides secure export and import of persistent volume contents over HTTP protocol.
* Modules that provide a CSI driver implementation for integration with various types of storage (software and hardware):

  * [`csi-ceph`](/modules/csi-ceph/);
  * [`csi-hpe`](/modules/csi-hpe/);
  * [`csi-huawei`](/modules/csi-huawei/);
  * [`csi-netapp`](/modules/csi-netapp/);
  * [`csi-nfs`](/modules/csi-nfs/);
  * [`csi-s3`](/modules/csi-s3/);
  * [`csi-scsi-generic`](/modules/csi-scsi-generic/);
  * [`csi-vsphere`](/modules/csi-vsphere/);
  * [`csi-csi-yadro-tatlin-unified`](/modules/csi-yadro-tatlin-unified/).

Only [local-path-provisioner module](local-path-provisioner.html) is currently described in this section. Documentation for the remaining Storage subsystem modules will be added as it becomes available.
