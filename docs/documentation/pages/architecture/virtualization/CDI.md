---
title: Containerized Data Importer (CDI)
permalink: en/architecture/virtualization/cdi.html
search: containerized-data-importer, containerized data importer, cdi
description: Architecture of the CDI component of virtualization module in Deckhouse Kubernetes Platform.
---

{% alert level="info" %}
A [fork](https://github.com/deckhouse/3p-containerized-data-importer) of CDI is used in the [`virtualization`](/modules/virtualization/) module. The [original CDI](https://github.com/kubevirt/containerized-data-importer) is a KubeVirt subproject. [KubeVirt](https://github.com/kubevirt/kubevirt) is an open-source project that allows you to launch, deploy, and manage virtual machines (VMs) using Kubernetes as an orchestration platform.
{% endalert %}

Containerized Data Importer (CDI) component of the [`virtualization`](/modules/virtualization/) module is a persistent storage management add-on for Kubernetes. It's primary goal is to provide a declarative way to build VM disks based on PersistentVolumeClaim (PVC) resources. CDI provides the ability to import VM images and disks into PVC volumes for use in KubeVirt-managed VM. The data can come from different sources:

- A URL address
- A container registry
- Another PVC (clone)
- A snapshot
- An upload from a client

CDI supports import of two types of data:

- **KubeVirt data**: Indicates that the file being imported should be treated as a KubeVirt VM disk. CDI will automatically decompress and convert the file from the supported format to `raw` or `qcow2` format (depending on the volume mode). It will also resize the disk to use all available space.
- **Archive data**: Indicates that the data is a TAR archive. Compression is not supported for archives. CDI will extract the contents of the archive into the volume, which can then be used with either a regular pod, or a VM using KubeVirt's filesystem feature.

CDI uses custom resources for disk management. The InternalVirtualizationDataVolume custom resource is an abstraction on top of the standard Kubernetes PVC and can be used to automate creation and population of a PVC with data.

## CDI architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

- The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
- Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the CDI component of [`virtualization`](/modules/virtualization/) module and its interactions with other components of DKP are shown in the following diagrams:

![Architecture of the CDI component of virtualization module](../../images/architecture/virtualization/c4-l2-virtualization-cdi.png)

## CDI components

CDI consists of the following components:

1. **Cdi-operator**: A Kubernetes operator that manages the CDI components lifecycle using an InternalVirtualizationCDI custom resource. Cdi-operator installs cdi-apiserver and cdi-deployment in the cluster and performs their configuration as well.

   It consists of the following containers:

   - **cdi-operator**:  Main container.
   - **proxy** (aka **kube-api-rewriter**): Sidecar container that performs modification of API requests passing through it, namely renaming the metadata of custom resources. This is necessary because KubeVirt components use API groups like `*.kubervirt.io`, and other components of the [`virtualization`](/modules/virtualization/) module use similar resources, but with API groups like `*.virtualization.deckhouse.io`. Kube-api-rewriter is a gateway that proxies requests between controllers that manage resources from different API groups.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to the metrics of the proxy container. It is an [open-source project](https://github.com/brancz/kube-rbac-proxy).

1. **Cdi-apiserver**: [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/), which is used to validate and mutate Kubernetes API resources through the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanisms. Cdi-apiserver implements validating and mutating webhooks for the following types of resources:

   - PersistentVolumeClaim: A standard Kubernetes API resource.
   - InternalVirtualizationDataVolume: An abstraction on top of the standard Kubernetes PVC for creating VM disks.
   - InternalVirtualizationCDI: A custom resource used by the cdi-operator to install and configure CDI components.
   - InternalVirtualizationDataImportCron: It defines a cron task for importing disk images as PVCs.
   - VolumeImportSource: It defines the sources for importing disks.

   It consists of the following containers:

   - **cdi-apiserver**: Main container.
   - **proxy** (aka **kube-api-rewriter**): Sidecar container that performs modification of API requests passing through it (described above).
   - **kube-rbac-proxy**: Sidecar container providing authorized access to the metrics of the cdi-apiserver and proxy containers (described above).

1. **Cdi-deployment** (aka **cdi-controller**): A controller that performs the following operations with InternalVirtualizationDataVolumes:

   - Import of VM images and disks into PVC volumes for use in KubeVirt managed VMs.
   - Disk cloning (importing into PVC from other PVC volumes or snapshots).
   - Synchronization of PVC with the corresponding InternalVirtualizationDataVolumes custom resources.

   To perform some of the above operations, the controller creates and launches temporary pods:

   - **cdi-importer**: For importing images and VM disks. Cdi-importer also converts images depending on the type of target PVC:

     - To `raw` format, if the `Block` volume mode is set for the PVC.
     - To `qcow2` format, if the `Filesystem` volume mode is set for the PVC.

   - **cdi-cloner**: For disks and snapshots cloning.

   It consists of the following containers:

   - **cdi-deployment**: Main container, built upon the [cdi-controller](https://github.com/deckhouse/3p-containerized-data-importer/blob/main/cmd/cdi-controller/controller.go);
   - **proxy** (aka **kube-api-rewriter**): Described above.
   - **kube-rbac-proxy**: Sidecar container providing authorized access to the metrics of the cdi-deployment and proxy containers (described above).

## CDI interactions

CDI interacts with the following components:

1. **Kube-apiserver**:

   - Watches for InternalVirtualizationCDI and PersistentVolumeClaim resources.
   - Authorizes requests for metrics.

1. **KubeVirt**: Provides the ability to populate PVCs with VM images and disks to use them in KubeVirt managed VMs.

1. **DVCR (Deckhouse Virtualization Container Registry)**: Uses the container registry as a source for importing VM images and disks.

The following external components interact with the CDI component:

1. **Kube-apiserver**: Validates/mutates CDI component custom resources as well as PersistentVolumeClaim standard resources.

1. **Prometheus-main**: Collects CDI components metrics.
