---
title: "Cloud provider — OpenStack"
description: "Cloud resource management in Deckhouse Kubernetes Platform using OpenStack."
---

The `cloud-provider-openstack` module integrates Deckhouse Kubernetes Platform with [OpenStack](https://www.openstack.org/)-based clouds. It allows the [`node-manager`](/modules/node-manager/) module to use OpenStack resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-openstack` module:

- Managing OpenStack resources via `cloud-controller-manager`:
  - updates OpenStack server and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in OpenStack;
  - creates load balancers (Octavia) for Services of the LoadBalancer type.
- Provisioning block disks via the Cinder CSI driver (`cinder.csi.openstack.org`). Manila (filesystem) is not supported. The Cinder CSI driver supports re-authentication in OpenStack with service catalog refresh, which improves resilience for long-running pods with volumes.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM) or Cluster API (CAPI). Virtual machine parameters are set in the [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used. The network mode depends on the [podNetworkMode](/modules/cloud-provider-openstack/configuration.html#parameters-podnetworkmode) parameter.

{% alert level="warning" %}
The module is migrating CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing [NodeGroups](/modules/node-manager/cr.html#nodegroup) continue to use MCM, while new ones are created with CAPI by default. For migrating existing groups, see [How to migrate node groups to Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi).
{% endalert %}
