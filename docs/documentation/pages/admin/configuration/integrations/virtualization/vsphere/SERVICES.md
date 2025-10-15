---
title: Integration with VMware vSphere services
permalink: en/admin/integrations/virtualization/vsphere/services.html
---

Deckhouse Kubernetes Platform integrates with VMware vSphere infrastructure and uses [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resources
to describe the specifications of virtual machines created as part of the Kubernetes cluster.

Key features:

- Provisioning and removal of virtual machines via the vCenter API.
- Node placement across multiple clusters ([`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones)) and datacenters ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region)).
- Use of VM templates with `cloud-init`.
- Support for networks with DHCP, static addressing, and additional interfaces.
- Storage management: provisioning root disks and PVCs based on Datastore or CNS disks.
- Support for incoming traffic load balancing:
  - Via external load balancers.
  - Via MetalLB (in BGP mode).

{% alert level="info" %}
It is possible to create a hybrid cluster with nodes running on both vSphere and bare metal.
{% endalert %}
