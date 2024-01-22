---
title: "Cloud provider — VMware vSphere: FAQ"
---

## How do I create a hybrid cluster?

A hybrid cluster combines bare metal and vSphere nodes. To create such a cluster, you will need an L2 network between all nodes of the cluster.

To create a hybrid cluster, you need to:

1. Delete flannel from kube-system:  `kubectl -n kube-system delete ds flannel-ds`.
2. Enable the module and specify the necessary [parameters](configuration.html#parameters).

> **Caution!** Cloud-controller-manager synchronizes vSphere and Kubernetes states by deleting Kubernetes nodes that are not in vSphere. This is not always required in a hybrid cluster. Therefore, if a Kubernetes node is started without the `--cloud-provider=external` parameter, it is automatically ignored. Deckhouse assigns `static://` to nodes in `.spec.providerId`, so cloud-controller-manager ignores such nodes.
