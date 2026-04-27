---
title: Cloud-provider-aws module
permalink: en/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-aws.html
search: cloud-provider-aws, cloud provider aws, amazon web services
description: Architecture of the cloud-provider-aws module in Deckhouse Kubernetes Platform.
---

The `cloud-provider-aws` module is responsible for interacting with the [Amazon Web Services](https://aws.amazon.com/) cloud resources. It allows the [`node-manager`](/modules/node-manager/) module to use AWS resources for provisioning nodes for the specified [node group](/modules/node-manager/cr.html#nodegroup).

For more details about the module configuration, refer to [the corresponding documentation](/modules/cloud-provider-aws/) section.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`cloud-provider-aws`](/modules/cloud-provider-aws/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Cloud-provider-aws architecture](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-provider-aws.png)

## Module components

The module consists of the following components:

1. **Cloud-controller-manager**: It is an implementation of [cloud controller manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) for AWS. It provides interaction with the AWS cloud and performs the following functions:

   * Implements a 1:1 relationship between a Node resource in Kubernetes and a VM in a cloud provider. To do this:

     * It fills the `spec.providerId` and `NodeInfo` fields of the Node resource.
     * It checks for a VM in the cloud and deletes the Node resource in the cluster if it is missing.

   * When creating a LoadBalancer Service resource in Kubernetes, it creates a load balancer in the cloud that routes traffic from outside into the cluster nodes.
   * Creates network routes for the `PodNetwork` network in the cloud.

   For more details about cloud-controller-manager, refer to [the Kubernetes documentation](https://kubernetes.io/docs/concepts/architecture/cloud-controller/).

   It consists of a single container:

   * **aws-cloud-controller-manager**.

1. **Cloud-data-discoverer**: It is responsible for collecting data from the cloud provider's API and providing it as a `kube-system/d8-cloud-provider-discovery-data` Secret. This secret contains the parameters of a specific cloud used by other components of the `cloud-provider-aws` module.

   It consists of the following containers:

   * **cloud-data-discoverer**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the cloud-data-discoverer metrics.

1. **CSI driver (aws)**: It is an implementation of the CSI driver for AWS. To study the `cloud-provider-*` CSI driver typical architecture, refer to [the corresponding documentation](../infrastructure/csi-driver.html) section.

1. **Node-termination-handler**: [AWS Node Termination Handler](https://github.com/aws/aws-node-termination-handler). It is responsible for gracefully handling the termination of EC2 instances in the Kubernetes control plane.

   Node-termination-handler processes the following AWS events:
   * [EC2 maintenance events](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/monitoring-instances-status-check_sched.html)
   * [EC2 Spot interruptions](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-interruptions.html)
   * [ASG Scale-In](https://docs.aws.amazon.com/autoscaling/ec2/userguide/AutoScalingGroupLifecycle.html#as-lifecycle-scale-in)
   * [ASG AZ Rebalance](https://docs.aws.amazon.com/autoscaling/ec2/userguide/ec2-auto-scaling-capacity-rebalancing.html)
   * [EC2 Instance Termination](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/terminating-instances.html)

   Node-termination-handler performs drain and/or cordon of the corresponding node after receiving of the specified events.

   It consists of a single container:

   * **node-termination-handler**.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

    * Watches for PersistentVolumeClaim and VolumeAttachment custom resources.
    * Creates the `kube-system/d8-cloud-provider-discovery-data` Secret.
    * Synchronizes Kubernetes nodes with cloud VMs.
    * Watches for LoadBalancer services.
    * Authorizes the requests for metrics.

1. **Amazon Web Services**:

    * Collects cloud parameters.
    * Gets `ProviderId` and other information about the VMs that are cluster nodes.
    * Manages load balancers.
    * Manages network routes for `PodNetwork` network.
    * Collects cloud events of EC2 instance termination.
    * Manages disks.

The following external components interact with the module:

* **Prometheus-main**: Collects cloud-data-discoverer metrics.

Indirect interactions:

1. The `cloud-provider-aws` module provides [`node-manager`](/modules/node-manager/) with following artifacts:

   * Provider-specific custom resource templates to be used by `cloud-provider-aws` to create VMs in the cloud.
   * The `kube-system/d8-node-manager-cloud-provider` Secret, which contains all the necessary settings to connect to the cloud and to create CloudEphemeral nodes. These settings are registered in the provider-specific custom resources created based on the templates mentioned above.

1. The `cloud-provider-aws` module provides Terraform/OpenTofu components for AWS cloud used when building the [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) executable file for the [`terraform-manager`](/modules/terraform-manager/) module, such as:

   * Terraform/OpenTofu provider.
   * Terraform modules.
   * Layouts: Set of cloud placement schemes, which define how the basic infrastructure is created, how and with which additional characteristics should nodes be created for this placement. For example, for one scheme, nodes may have public IP addresses, but they will not for the other. Each layout should have three modules:

     * `base-infrastructure`: Basic infrastructure (for example, creation of networks), can also be empty
     * `master-node`
     * `static-node`.
