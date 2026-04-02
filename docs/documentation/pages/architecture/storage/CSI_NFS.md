---
title: Csi-nfs module
permalink: en/architecture/storage/csi-nfs.html
search: csi-nfs, nfs
description: Architecture of the csi-nfs module in Deckhouse Kubernetes Platform.
---

The `csi-nfs` module is designed to manage NFS-based volumes. It enables creating StorageClasses in Kubernetes using the NFSStorageClass resource.

For more details about module, refer to [the module documentation section](/modules/csi-nfs/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`csi-nfs`](/modules/csi-nfs/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagrams:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![sds-local-volume module architecture](../../../images/architecture/storage/c4-l2-csi-nfs.png)

## Module components

The module consists of the following components:

1. **Controller**: It reconciles [NFSStorageClass](/modules/csi-nfs/stable/cr.html#nfsstorageclass) custom resources. NFSStorageClass is a Kubernetes custom resource that defines the configuration for Kubernetes StorageClass. The StorageClass being created uses `nfs.csi.k8s.io` provisioner. StorageClass configures connection settings to NFS-server, reclaim policy, volume binding mode, etc. These settings are used by the provisioner of CSI driver (`csi-nfs`) when managing NFS-based volumes.

   It consists of the following containers:

   * **controller**: Main container.
   * **webhook**: A sidecar container that implements a webhook server for validating NFSStorageClass custom resources, StorageClass resources.

2. **Sds-local-volume-scheduler-extender**: It consists of a single container. It is a kube-scheduler extender, which implements a scheduling logic specific for pods using NFS-based volumes. When planning, the rules of selecting nodes in NFSStorageClass are taken into account.

3. **CSI driver (`csi-nfs`)**: It is an implementation of the CSI driver for `nfs.csi.k8s.io` ([NFS CSI driver](https://github.com/kubernetes-csi/csi-driver-nfs)). To study the CSI driver (`csi-nfs`) architecture used in DKP, refer to [the CSI-driver architecture documentation section](../storage/csi-drivers/csi-driver-nfs.html).

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Watches for PersistentVolume, PersistentVolumeClaim, VolumeAttachment and StorageClass resources.
   * Reconciles NFSStorageClass custom resources.
   * Creates StorageClass resources.

The following external components interact with the module:

1. **Kube-apiserver**: Validates NFSStorageClass custom resources and StorageClass resources.

2. **Kube-scheduler**: Sends scheduling requests to the `csi-nfs-scheduler-extender` webhook for the pods used NFS-based volumes.
