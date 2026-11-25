---
title: Integration with IaaS
permalink: en/admin/integrations/integrations-overview.html
---

Deckhouse Platform provides built-in tools for integrating with [cloud providers](../../glossary.html#cloud-provider) and virtualization systems. For every supported provider, the platform:

- Orders virtual machines and connects them to the cluster as nodes, including when a [node group](/modules/node-manager/cr.html#nodegroup) scales automatically.
- Orders disks through the CSI driver of the provider, so that PersistentVolumes can be ordered in the cluster.
- Keeps node metadata up to date and removes from the cluster the nodes that no longer exist on the provider side.
- Enables CNI in new clusters, [`cni-cilium`](/modules/cni-cilium/) by default.

Some providers also create the base cluster infrastructure and load balancers for Service objects of the `LoadBalancer` type. The capabilities of each provider are listed in the overview of the integration with it.

[Cluster deployment](../../installing/) is the same in every cloud. Only the cloud preparation, such as creating a service account, and the contents of the configuration files differ.

Supported [cloud providers](./public/overview.html):

- Amazon Web Services (AWS)
- Google Cloud Platform (GCP)
- Microsoft Azure
- OpenStack
- OVH Cloud
- Selectel Cloud
- VK Cloud
- Yandex Cloud

Integration is also possible with [private clouds](./private/overview.html) based on the following solutions:

- VK Cloud
- OpenStack
- Basis Dynamix
- Huawei Cloud

In addition to cloud providers, integration is supported with the following [virtualization systems](./virtualization/overview.html):

- built-in virtualization
- VMware Cloud Director
- VMware vSphere
- zVirt

It is also possible to configure [hybrid clusters](./hybrid/overview.html).
