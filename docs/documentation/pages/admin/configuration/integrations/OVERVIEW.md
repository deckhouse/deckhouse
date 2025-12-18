---
title: Overview
permalink: en/admin/integrations/integrations-overview.html
---

Deckhouse Kubernetes Platform (DKP) provides built-in tools
for integrating with various cloud providers and virtualization systems.
These tools let you:

- Automatically use cloud infrastructure to provision virtual machines and connect them to the cluster.
- Deploy clusters in cloud environments.
- Scale resources as needed.
- Automatically create base infrastructure resources (such as networks, routes, etc.), if supported by the provider.

The workflow for working with different cloud providers is generally the same,
with only the preparation steps (such as creating a service account in the cloud) and configuration files differing.

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
- Huawei Cloud

In addition to cloud providers, integration is supported with the following [virtualization systems](./virtualization/overview.html):

- Deckhouse Virtualization Platform (DVP)
- VMware Cloud Director
- VMware vSphere
- zVirt

It is also possible to configure [hybrid clusters](./hybrid/overview.html).
