---
title: How to migrate node groups to Cluster API (CAPI)?
subsystems:
  - cluster_infrastructure
lang: en
---

Deckhouse Kubernetes Platform is migrating CloudEphemeral node management from Machine Controller Manager (MCM) to [Cluster API](https://cluster-api.sigs.k8s.io/) (CAPI).

Currently, CAPI is supported for the following cloud providers:

- [Yandex Cloud](/modules/cloud-provider-yandex/)
- [OpenStack](/modules/cloud-provider-openstack/)

After the provider starts supporting CAPI, existing CloudEphemeral [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources keep the MCM engine (`status.engine: MCM`). New NodeGroups are created with CAPI by default (`status.engine: CAPI`).

To check which engine manages a node group:

```shell
d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
```

## How to migrate

To switch a node group to CAPI, recreate the [NodeGroup](/modules/node-manager/cr.html#nodegroup): delete the existing group and create it again with the same configuration. New nodes will be managed by CAPI.

{% alert level="warning" %}
Recreating a NodeGroup causes the nodes of that group to be recreated. Plan the migration carefully and perform it during a maintenance window if necessary.
{% endalert %}

## Forcing MCM in case of problems

If CAPI does not work as expected, you can force MCM for a NodeGroup by setting the `node.deckhouse.io/use-mcm` annotation before the group is created (or when recreating it):

```shell
d8 k annotate nodegroup <NODE_GROUP_NAME> node.deckhouse.io/use-mcm=""
```

{% alert level="warning" %}
Using the `node.deckhouse.io/use-mcm` annotation is a temporary workaround and is not recommended. Prefer migrating node groups to CAPI.
{% endalert %}
