---
title: Managed service
description: A ready-to-use application service in the cluster, such as a database or a message queue, whose lifecycle is handled by Deckhouse Platform.
lang: en
---

Each managed service adds its own Kubernetes resources and controllers to the cluster. A user describes the desired state of the service in a namespaced resource, and the controller creates and updates the related objects: workload instances, Services, secrets and snapshots. The administrator manages cluster-wide resources, such as service classes, that define the allowed configurations and default values for user resources. Deckhouse Platform handles administration, scaling, backup and updates of the service.

For details, see [Managed services](admin/configuration/managed-services/).
