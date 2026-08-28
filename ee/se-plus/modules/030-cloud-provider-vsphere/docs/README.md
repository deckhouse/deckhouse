---
title: "Cloud provider — VMware vSphere"
description: "Managing cloud resources in Deckhouse Kubernetes Platform based on VMware vSphere."
---

The `cloud-provider-vsphere` module is responsible for interacting with the [VMware vSphere-based](https://www.vmware.com/products/vsphere.html) cloud resources. It allows the [node manager](/node-manager/) module to use vSphere resources for provisioning nodes for the specified [node group](/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-vsphere` module:

- Manages vSphere resources using the `cloud-controller-manager` module:
  - Creates network routes for the PodNetwork network on the vSphere side.
  - Keeps metadata of vSphere VirtualMachines and Kubernetes Nodes up to date. Removes Kubernetes nodes that no longer exist in vSphere.
- Provisions disks on vSphere datastores using the First-Class Disk mechanism via the `CSI storage` component.
- Registers with the [`node-manager`](/node-manager/) module so that [VsphereInstanceClass resources](cr.html#vsphereinstanceclass) can be used when configuring a [NodeGroup](/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
This module is transitioning CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing NodeGroups continue to use MCM, while newly created NodeGroups use CAPI by default. For the migration procedure for existing groups, see [How to migrate node groups to Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi).
{% endalert %}

{% alert level="info" %}
**vCenter tag parity for CAPI-managed VMs.** Under CAPI, every VM receives the `deckhouse-cluster-name/<clusterUUID>` tag (matching MCM behavior). The per-role tag `deckhouse-node-role/<nodeGroup>-<zone>` that MCM also attached is not yet reproduced by the CAPI pipeline — use Kubernetes node labels (`node.deckhouse.io/group`) to group nodes by NodeGroup instead. Full tag parity is planned as a follow-up.

**Placement fields on `VsphereInstanceClass` under CAPI.** `spec.resourcePool` is honored: when it is set, the module creates a per-NodeGroup `VSphereDeploymentZone` for every zone the NodeGroup spans, with `placementConstraint.resourcePool` set to the InstanceClass value, and points the NodeGroup's `MachineDeployment` at that DZ. `spec.datastore` is **not** honored — CAPV reads datastore from `VSphereFailureDomain.spec.topology.datastore`, which is one per zone and immutable via the FailureDomain webhook. Multiple NodeGroups in the same zone therefore share the zone-default datastore; to place NodeGroups on different datastores, use different zones.
{% endalert %}
