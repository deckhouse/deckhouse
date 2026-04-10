---
title: Cloud-provider-yandex module
permalink: en/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-yandex.html
search: cloud-provider-yandex, cloud provider yandex
description: Architecture of the cloud-provider-yandex module in Deckhouse Kubernetes Platform.
---

The `cloud-provider-yandex` module is responsible for interacting with the [Yandex Cloud](https://yandex.cloud/en) cloud resources. It allows the [`node-manager`](/modules/node-manager/) module to use Yandex Cloud resources for provisioning nodes for the specified [node group](/modules/node-manager/cr.html#nodegroup).

For more details about the module configuration, refer to the [corresponding documentation section](/modules/cloud-provider-yandex/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Cloud-provider-yandex architecture](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-yandex.png)

## Module components

The module consists of the following components:

1. **Cloud-controller-manager**: It is an implementation of [Cloud controller manager](https://kubernetes.io/ru/docs/concepts/architecture/cloud-controller/) for Yandex Cloud. It provides interaction with the Yandex Cloud and performs the following functions:

   * Implements a 1:1 relationship between a Node resource in Kubernetes and a VM in a cloud provider. To do this:

     * It fills the `spec.providerId` and `NodeInfo` fields of the Node resource.
     * It checks for a VM in the cloud and deletes the Node resource in the cluster if it is missing.

   * When creating a LoadBalancer Service resource in Kubernetes, it creates a load balancer in the cloud that routes traffic from outside into the cluster nodes.
   * Creates network routes for the `PodNetwork` network in the cloud.

   For more details about cloud-controller-manager, refer to the [Kubernetes documentation](https://kubernetes.io/docs/concepts/architecture/cloud-controller/).

   It consists of a single container:

   * **yandex-cloud-controller-manager**.

2. **Cloud-data-discoverer**: It is responsible for collecting data from the cloud provider's API and providing it as a `kube-system/d8-cloud-provider-discovery-data` Secret. This secret contains the parameters of a specific cloud used by other components of the `cloud-provider-yandex` module.

   It consists of the following containers:

   * **cloud-data-discoverer**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the cloud-data-discoverer metrics.

3. **Cloud-metrics-exporter**: It is responsible for collecting metrics from the cloud provider's API and providing them in Prometheus format.

   It consists of the following containers:

   * **cloud-metrics-exporter**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the cloud-metrics-exporter metrics.

4. **CSI driver (yandex)**: It is an implementation of the CSI driver for Yandex Cloud. To study the `cloud-provider-*` CSI driver typical architecture, refer to the [corresponding documentation page](../infrastructure/csi-driver.html).

   CSI driver (yandex) does not support snapshots. For this reason, the `csi-controller` Pod does not include the snapshotter ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter )) sidecar container.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

    * Watches for PersistentVolumeClaim and VolumeAttachment custom resources.
    * Creates the `kube-system/d8-cloud-provider-discovery-data` Secret.
    * Synchronizes Kubernetes nodes with cloud VMs.
    * Watches for LoadBalancer services.
    * Authorizes the requests for metrics.

2. **Yandex Cloud**:

    * Collects cloud parameters.
    * Gets `ProviderId` and other information about the VMs that are cluster nodes.
    * Manages load balancers.
    * Manages disks.
    * Manages network routes for `PodNetwork` network.
    * Collects cloud service metrics.

The following external components interact with the module:

1. **Prometheus-main**:

   * Collects cloud-data-discoverer metrics.
   * Collects cloud provider's metrics using cloud-metrics-exporter.

Indirect interactions:

1. The `cloud-provider-yandex` module provides [`node-manager`](/modules/node-manager/) with following artifacts:

   * Provider-specific custom resource templates to be used by `cloud-provider-yandex` to create VMs in the cloud.
   * The `kube-system/d8-node-manager-cloud-provider` Secret, which contains all the necessary settings to connect to the cloud and to create CloudEphemeral nodes. These settings are registered in the provider-specific custom resources created based on the templates mentioned above.

2. The `cloud-provider-yandex` module provides Terraform/OpenTofu components for Yandex Cloud used when building the [dhctl](https://github.com/deckhouse/deckhouse/tree/main/dhctl) executable file for the [`terraform-manager`](/modules/terraform-manager/) module, such as:

   * Terraform/OpenTofu provider.
   * Terraform modules.
   * Layouts: Set of cloud placement schemes, which define how the basic infrastructure is created, how and with which additional characteristics should nodes be created for this placement. For example, for one scheme, nodes may have public IP addresses, but they will not for the other. Each layout should have three modules:

     * `base-infrastructure`: Basic infrastructure (for example, creation of networks), can also be empty.
     * `master-node`
     * `static-node`
