---
title: Csi-vsphere module
permalink: en/architecture/storage/csi-vsphere.html
search: csi-vsphere, vmware vsphere
description: Architecture of the csi-vsphere module in Deckhouse Kubernetes Platform.
---

The `csi-vsphere` module provides [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) support for VMware vSphere environments, enabling dynamic provisioning and management of persistent storage volumes in Kubernetes clusters running on vSphere infrastructure.

For more details about the module configuration, refer to [the corresponding documentation](/modules/csi-vsphere/) section.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`csi-vsphere`](/modules/csi-vsphere/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![csi-vsphere architecture](../../../../images/architecture/cluster-and-infrastructure/c4-l2-csi-vsphere.png)

## Module components

The module consists of the following components:

1. **Cloud-data-discoverer**: It is responsible for collecting data from the cloud provider's API and providing it as a `kube-system/d8-cloud-provider-discovery-data` Secret. This secret contains the parameters of a specific cloud used CSI driver for volume management.

   It consists of the following containers:

   * **cloud-data-discoverer**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the cloud-data-discoverer metrics.

1. **CSI driver (vsphere)**: It is an implementation of the CSI driver for VMware vSphere. To study the `csi-vsphere` CSI driver architecture, refer to [the corresponding documentation](../storage/csi-drivers/csi-vsphere.html) section.

   CSI driver (vsphere) does not support snapshots. For this reason, the `csi-controller` Pod does not include the snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)) sidecar container.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

    * Watches for PersistentVolumeClaim and VolumeAttachment custom resources.
    * Creates the `kube-system/d8-cloud-provider-discovery-data` Secret.
    * Creates StorageClass resources for each discovered datastore.
    * Authorizes the requests for metrics.

1. **VMware vSphere**:

    * Collects cloud parameters.
    * Manages disks.

The following external components interact with the module:

* **Prometheus-main**: Collects cloud-data-discoverer metrics.
