---
title: "Cloud provider — OpenStack: configuration"
---

The module is automatically enabled for all cloud clusters deployed in OpenStack.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the OpenStack-based cloud provider, the instance class is the [`OpenStackInstanceClass`](cr.html#openstackinstanceclass) custom resource that stores specific parameters of the machines.

The module settings are set automatically based on the placement strategy chosen. In most cases, you do not have to configure the module manually.

If you need to configure a module because, say, you have a bare metal cluster and you need to enable additional instances from vSphere, then refer to the [How to configure a Hybrid cluster in vSphere](faq.html#how-do-i-create-a-hybrid-cluster) section.

> **Note!** If the parameters provided below are changed, the **existing `Machines` are NOT redeployed** (new `Machines` will be created with the updated parameters). Redeployment is only performed when `NodeGroup` and `OpenStackInstanceClass` parameters are changed. You can learn more in the [node-manager](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration) module's documentation.
To authenticate using the `user-authn` module, you need to create a new `Generic` application in the project's Crowd.

{% include module-settings.liquid %}

## List of required OpenStack services

A list of OpenStack services required for Deckhouse Kubernetes Platform to work in OpenStack:

| Service | API Version |
| :------------- | :------------- |
| Identity (Keystone) | v3 |
| Compute (Nova) | v2 |
| Network (Neutron) | v2 |
| Block Storage (Cinder) | If the Load Balancer ordering functionality will be used: v3 |
| Load Balancing (Octavia) | v2 |
