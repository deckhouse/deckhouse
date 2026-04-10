---
title: Storage subsystem
permalink: en/architecture/storage/
search: storage, storage subsystem
description: Architecture of the Storage subsystem in Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This subsection describes the architecture of the Storage subsystem of Deckhouse Kubernetes Platform (DKP).

The Storage subsystem includes the following modules:

* [`local-path-provisioner`](/modules/local-path-provisioner/): Provides the local storage on Kubernetes nodes using `HostPath` volumes and creates StorageClass resources to manage the allocation of local storage.
* [`snapshot-controller`](/modules/snapshot-controller/): Enables snapshot support for compatible CSI-drivers in the Kubernetes cluster.
* [`sds-local-volume`](/modules/sds-local-volume/): Manages the local block storage based on LVM and enables creating StorageClass resources in Kubernetes using the [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resource.
* [`sds-node-configurator`](/modules/sds-node-configurator/): Manages block devices and LVM on Kubernetes cluster nodes through [Kubernetes custom resources](/modules/sds-node-configurator/stable/cr.html).
* [`sds-replicated-volume`](/modules/sds-replicated-volume/): Manages replicated block storage based on DRBD. LINSTOR is used as a control plane.
* [`storage-volume-data-manager`](/modules/storage-volume-data-manager/): Provides secure export and import of persistent volume contents over HTTP protocol.
* Modules that provide a CSI driver implementation for integration with various types of storage (software and hardware):

  * [`csi-ceph`](/modules/csi-ceph/)
  * [`csi-hpe`](/modules/csi-hpe/)
  * [`csi-huawei`](/modules/csi-huawei/)
  * [`csi-netapp`](/modules/csi-netapp/)
  * [`csi-nfs`](/modules/csi-nfs/)
  * [`csi-s3`](/modules/csi-s3/)
  * [`csi-scsi-generic`](/modules/csi-scsi-generic/)
  * [`csi-vsphere`](/modules/csi-vsphere/)
  * [`csi-yadro-tatlin-unified`](/modules/csi-yadro-tatlin-unified/)

The following modules are currently described in this section:

* [`local-path-provisioner`](local-path-provisioner.html)
* [`snapshot-controller`](snapshot-controller.html)
* [`sds-local-volume`](sds-local-volume.html)
* [`sds-node-configurator`](sds-node-configurator.html)

Documentation for the remaining Storage subsystem modules will be added as it becomes available.
