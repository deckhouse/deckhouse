---
title: Integration with DVP
permalink: en/admin/integrations/virtualization/dvp/services.html
---

Deckhouse Kubernetes Platform integrates with the DVP infrastructure and uses [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resources to define the characteristics of virtual machines created as part of the cluster.

Key features:

- Management of DVP resources via the `cloud-controller-manager module`
- Provisioning of disks using the CSI storage component
- Integration with the [`node-manager`](/modules/node-manager/) module to support DVPInstanceClass when defining a [NodeGroup](/modules/node-manager/cr.html#nodegroup)

{% alert level="info" %}
The module is automatically enabled for all cloud clusters deployed in DVP.
The module has no configurable settings.
{% endalert %}
