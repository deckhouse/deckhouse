---
title: "Cloud provider — VMware vSphere: FAQ"
---

## How do I create a hybrid cluster?

A hybrid cluster combines bare metal and vSphere nodes. To create such a cluster, you will need an L2 network between all nodes of the cluster.

{% alert level="info" %}
The Deckhouse Kubernetes Platform allows to set a prefix for the names of CloudEphemeral nodes added to a hybrid cluster with Static master nodes.
To do this, use the [`instancePrefix`](../node-manager/configuration.html#parameters-instanceprefix) parameter of the `node-manager` module. The prefix specified in the parameter will be added to the name of all CloudEphemeral nodes added to the cluster. It is not possible to set a prefix for a specific NodeGroup.
{% endalert %}

To create a hybrid cluster, you need to:

1. Delete flannel from kube-system:  `d8 k -n kube-system delete ds flannel-ds`.
2. Enable the module and specify the necessary [parameters](configuration.html#parameters).

> **Caution!** Cloud-controller-manager synchronizes vSphere and Kubernetes states by deleting Kubernetes nodes that are not in vSphere. In a hybrid cluster, such behavior does not always make sense. That is why cloud-controller-manager automatically skips Kubernetes nodes that do not have the `--cloud-provider=external` parameter set (Deckhouse inserts `static://` to nodes in `.spec.providerID`, and cloud-controller-manager ignores them).
