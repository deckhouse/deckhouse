---
title: How to migrate node groups to Cluster API (CAPI)?
subsystems:
  - cluster_infrastructure
lang: en
---

Deckhouse Kubernetes Platform is migrating CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI).

At the moment, migration from MCM to CAPI is supported for the following cloud providers:

- [Yandex Cloud](/modules/cloud-provider-yandex/)
- [OpenStack](/modules/cloud-provider-openstack/)

After CAPI support is introduced for a cloud provider, existing [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#adding-cloudephemeral-nodes-to-a-cloud-cluster) node groups continue to use MCM (`status.engine: MCM`). New node groups use CAPI by default (`status.engine: CAPI`).

To check which management mechanism is used for a node group, run the following command:

```shell
d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

To migrate a node group from MCM to CAPI:

1. Create a new [NodeGroup](/modules/node-manager/cr.html#nodegroup) of the [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#adding-cloudephemeral-nodes-in-a-cloud-cluster) type with the required configuration. Do not set the `node.deckhouse.io/use-mcm` annotation — otherwise the group will remain on MCM.

1. Make sure the new group is managed by CAPI ([`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine)):

   ```shell
   d8 k get nodegroup <NODE_GROUP_NAME> -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
   ```

1. Wait until the nodes in the new group become `Ready`:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<NODE_GROUP_NAME>
   ```

1. Move workloads from the old group to the new one.

   For example, update application [`nodeSelector`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) and [`tolerations`](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) so that pods are scheduled onto nodes of the new group, or use another workload placement method adopted in your infrastructure. For details on allocating nodes for workloads, see [Allocating nodes for specific workloads](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#allocating-nodes-for-specific-workloads).

1. Make sure the migrated workloads are running successfully on the new group nodes and that no required pods remain on the old group nodes.
1. Delete the old NodeGroup.

{% alert level="warning" %}
Deleting a NodeGroup deletes all nodes in that group. Before deleting the old group, make sure the required workloads have been moved to the new group and are running correctly.
{% endalert %}

If necessary, you can force a node group to be created using MCM. To do this, add the `node.deckhouse.io/use-mcm` annotation before creating the NodeGroup (or before recreating it):

```shell
d8 k annotate nodegroup <NODE_GROUP_NAME> node.deckhouse.io/use-mcm="true"
```

For example:

```shell
d8 k annotate nodegroup worker node.deckhouse.io/use-mcm="true"
```

{% alert level="warning" %}
The `node.deckhouse.io/use-mcm` annotation is a temporary workaround and its use is not recommended. Migrating node groups to CAPI is the preferred approach.
{% endalert %}
