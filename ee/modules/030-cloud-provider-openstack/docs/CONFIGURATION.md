---
title: "Cloud provider — OpenStack: configuration"
---

The module is automatically enabled for all cloud clusters deployed in OpenStack.

You can configure the number and parameters of ordering machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the OpenStack-based cloud provider, the instance class is the [`OpenStackInstanceClass`](cr.html#openstackinstanceclass) custom resource that stores specific parameters of the machines.

## Parameters

<!-- SCHEMA -->
