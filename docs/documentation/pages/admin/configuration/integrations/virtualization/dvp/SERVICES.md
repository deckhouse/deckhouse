---
title: Integration with Deckhouse Virtualization Platform cloud
permalink: en/admin/integrations/virtualization/dvp/services.html
---

Deckhouse Platform integrates with the virtualization infrastructure and uses [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) resources to define the characteristics of virtual machines created as part of the cluster.

Key features:

- Management of virtualization resources via the `cloud-controller-manager module`
- Provisioning of disks using the CSI storage component
- Integration with the [`node-manager`](/modules/node-manager/) module to support DVPInstanceClass when defining a [NodeGroup](/modules/node-manager/cr.html#nodegroup)

{% alert level="info" %}
The virtualization integration is enabled automatically for all cloud clusters deployed in virtualization.
No additional configuration is required.
{% endalert %}
