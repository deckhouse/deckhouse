---
title: "Cloud provider — DVP"
description: "Deckhouse Kubernetes Platform integration with the Deckhouse Virtualization Platform. Deployment of DKP clusters on top of the DVP."
---

The `cloud-provider-dvp` module is responsible for interacting with the [DVP](https://deckhouse.io/products/virtualization-platform/) cloud resources. It allows the [`node-manager`](/modules/node-manager/) module to use DVP resources for provisioning nodes for the specified [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Key features of the `cloud-provider-dvp` module:

- Managing DVP resources using the `cloud-controller-manager` module.
- Provisioning disks using the `CSI storage` component.
- Integrating with the [`node-manager`](/modules/node-manager/) module so that [DVPInstanceClasses](cr.html#dvpinstanceclass) can be used when defining a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
If the cluster was installed with the DVPClusterConfiguration schema, migration to ModuleConfig-based configuration is required.
Until the migration is completed, the `D8CloudProviderDVPMigrationPending` alert may fire and Deckhouse updates may be blocked.

See: [How to migrate a cloud provider to ModuleConfig-based configuration](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).
{% endalert %}
