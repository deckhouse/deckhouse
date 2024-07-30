---
title: "Cloud provider — VMware vSphere: provider configuration"
description: Settings of the Deckhouse cloud provider for VMware vSphere.
---

> If the cluster control plane is hosted on a virtual machines or bare-metal servers, the cloud provider uses the settings from the `cloud-provider-vsphere` module in the Deckhouse configuration. Otherwise, if the cluster control plane is hosted in a cloud, the cloud provider uses the [VsphereClusterConfiguration](#vsphereclusterconfiguration) structure for configuration.
>
> Additional info about [Vsphere Cloud Load Balancers](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/pkg/cloudprovider/vsphere/loadbalancer).

<!-- SCHEMA -->
