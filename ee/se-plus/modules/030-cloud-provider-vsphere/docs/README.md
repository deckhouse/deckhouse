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

**Placement fields on `VsphereInstanceClass` under CAPI.** `spec.datastore` and `spec.resourcePool` are ignored for CAPI-managed NodeGroups — CAPV overrides them from the resolved `VSphereDeploymentZone` / `VSphereFailureDomain` topology on every reconcile. Per-InstanceClass override via extra DeploymentZones is planned as a follow-up.
{% endalert %}
