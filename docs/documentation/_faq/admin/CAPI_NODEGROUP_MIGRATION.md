---
title: How to migrate node groups to Cluster API (CAPI)?
subsystems:
  - cluster_infrastructure
lang: en
---

Deckhouse Kubernetes Platform is migrating CloudEphemeral node management from Machine Controller Manager (MCM) to Cluster API (CAPI).

At the moment, CAPI is supported for the following cloud providers:

- [Yandex Cloud](/modules/cloud-provider-yandex/);
- [OpenStack](/modules/cloud-provider-openstack/).

After CAPI support becomes available, existing [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#adding-cloudephemeral-nodes-in-a-cloud-cluster) node groups continue to use MCM (`status.engine: MCM`). Newly created node groups use CAPI by default (`status.engine: CAPI`).

To check which node management engine is used for a node group, run:

```shell
d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

## How to migrate

To migrate a node group to CAPI, recreate the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource: delete the existing resource and create it again using the same configuration. After that, new CloudEphemeral nodes will be created and managed by CAPI.

{% alert level="warning" %}
Recreating a NodeGroup recreates all nodes in that group. Plan the migration in advance and, if necessary, perform it during a maintenance window.
{% endalert %}

## Forcing NodeGroup creation with MCM

If necessary, you can force a node group to be created using MCM. To do this, add the `node.deckhouse.io/use-mcm` annotation before creating the NodeGroup (or before recreating it):

```shell
d8 k annotate nodegroup <NODE_GROUP_NAME> node.deckhouse.io/use-mcm=""
```

For example:

```shell
d8 k annotate nodegroup worker node.deckhouse.io/use-mcm=""
```

{% alert level="warning" %}
The `node.deckhouse.io/use-mcm` annotation is a temporary workaround and its use is not recommended. Migrating node groups to CAPI is the preferred approach.
{% endalert %}
