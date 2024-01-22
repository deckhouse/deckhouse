---
title: "Cloud provider — OpenStack: configuration"
---

The module is automatically enabled for all cloud clusters deployed in OpenStack.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). 

In the case of the OpenStack-based cloud provider, the instance class is the [`OpenStackInstanceClass`](cr.html#openstackinstanceclass) custom resource that stores specific parameters of the machines.

The module settings are set automatically based on the placement strategy chosen. In most cases, you do not have to configure the module manually.

A hybrid cluster consists of bare metal nodes and vSphere nodes combined into one cluster. To create such a cluster, it is necessary to have an L2 network between all nodes of the cluster. To configure a bare-metal cluster with additional OpenStack instances, follow these steps:

1. Delete flannel from cube-system: kubectl -n cube-system deleted s flannel-ds.

2. Turn on the module and specify the necessary parameters.

> **Important!** `Cloud-controller-manager` synchronizes the state between vSphere and Kubernetes, removing nodes from Kubernetes that are not in vSphere. In a hybrid cluster, this behavior is not always necessary. If the Kybernetes node is not started with the `--cloud-provider=external` parameter, it is automatically ignored (Deckhouse assigns static:// to nodes in `.spec.providerId`, and `cloud-controller-manager` ignores such nodes).

> **Note!** If the parameters provided below are changed, the **existing `Machines` are NOT redeployed** (new `Machines` will be created with the updated parameters). Redeployment is only performed when `NodeGroup` and `OpenStackInstanceClass` parameters are changed. You can learn more in the [node-manager](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration) module's documentation.
To authenticate using the `user-authn` module, you need to create a new `Generic` application in the project's Crowd.

{% include module-settings.liquid %}
