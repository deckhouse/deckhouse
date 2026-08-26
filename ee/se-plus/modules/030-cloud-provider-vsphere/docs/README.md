---
title: "Cloud provider — VMware vSphere"
description: "Cloud resource management in Deckhouse Kubernetes Platform using VMware vSphere."
---

The `cloud-provider-vsphere` module integrates Deckhouse Kubernetes Platform with [VMware vSphere](https://www.vmware.com/products/vsphere.html). It allows the [`node-manager`](/modules/node-manager/) module to use vSphere resources when provisioning nodes for a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Features of the `cloud-provider-vsphere` module:

- Managing vSphere resources via `cloud-controller-manager`:
  - creates network routes for the `PodNetwork` network on the vSphere side;
  - updates virtual machine and Kubernetes node metadata and removes from Kubernetes nodes that no longer exist in vSphere.
- Provisioning disks via CSI on datastore. By default, CNS volumes with online resize are used. First-Class Disk (FCD) mode is available as legacy and is configured with the [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag) parameter.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](/products/kubernetes-platform/documentation/v1/architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vsphere.html#module-interactions) `terraform-provider-vsphere`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM). Virtual machine parameters are set in the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource.
- Registering with [`node-manager`](/modules/node-manager/) so that [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) can be used when describing a [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Enabling CNI for new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.

{% alert level="warning" %}
This module is transitioning CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI). Existing NodeGroups continue to use MCM, while newly created NodeGroups use CAPI by default. For the migration procedure for existing groups, see [How to migrate node groups to Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi).
{% endalert %}

{% alert level="info" %}
**vCenter tag parity for CAPI-managed VMs.** Under CAPI, every VM receives the `deckhouse-cluster-name/<clusterUUID>` tag (matching MCM behavior). The per-role tag `deckhouse-node-role/<nodeGroup>-<zone>` that MCM also attached is not yet reproduced by the CAPI pipeline — use Kubernetes node labels (`node.deckhouse.io/group`) to group nodes by NodeGroup instead. Full tag parity is planned as a follow-up.

**Placement fields on `VsphereInstanceClass` under CAPI.** `spec.datastore` and `spec.resourcePool` are ignored for CAPI-managed NodeGroups — CAPV overrides them from the resolved `VSphereDeploymentZone` / `VSphereFailureDomain` topology on every reconcile. Per-InstanceClass override via extra DeploymentZones is planned as a follow-up.
{% endalert %}
