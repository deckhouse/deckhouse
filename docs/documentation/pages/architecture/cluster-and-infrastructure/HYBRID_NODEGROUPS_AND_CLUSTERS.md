---
title: Hybrid node group and cluster management
permalink: en/architecture/cluster-and-infrastructure/node-management/hybrid-nodegroups-and-clusters.html
search: hybrid cluster, hybrid node group
description: Architecture of hybrid node groups and hybrid clusters in Deckhouse Kubernetes Platform.
---

It is important to distinguish between hybrid node groups and hybrid clusters:

- **Hybrid node groups** may include both Static nodes deployed in the cloud and servers deployed in the customer's on-premises data center (bare metal or virtual machines).

  For example, the primary workload may run on bare-metal servers, while cloud instances are used as a scalable addition during peak loads. Since nodes of the same type (Static) are used both in the customer's data center and in the cloud, the architecture of the [`node-manager`](/modules/node-manager/) module in this case corresponds to the [Static node variant](static-nodes.html). The only mandatory requirement is the presence of L3 connectivity between the on-premises data center and the cloud. For example, [Yandex Cloud](https://yandex.cloud/) provides a [Cloud Interconnect](https://yandex.cloud/en/docs/interconnect/concepts/) mechanism.

- **Hybrid clusters** are DKP clusters that simultaneously include:

  - NodeGroup with Static nodes deployed in the customer's on-premises data center (bare metal or virtual machines).
  - NodeGroup with CloudEphemeral nodes deployed in the cloud.

  In this case, the components required to manage both node types are deployed. The architecture of the [`node-manager`](/modules/node-manager/) module corresponds to the [Static node variant](static-nodes.html). In addition, components required for CloudEphemeral nodes are deployed:

  - **cloud-provider**: Provides interaction with the cloud infrastructure. A configured provider for the corresponding cloud is required. It also includes csi-driver and cloud-controller-manager.
  - **cluster-autoscaler**: Provides automatic scaling of cluster nodes.

  As with hybrid node groups, hybrid clusters require L3 connectivity between the on-premises data center and the cloud.
