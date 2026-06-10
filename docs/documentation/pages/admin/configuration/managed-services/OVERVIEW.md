---
title: "Overview"
permalink: en/admin/configuration/managed-services/
description: "Administration of managed services in Deckhouse Kubernetes Platform"
---

Managed services in Deckhouse Kubernetes Platform let you run ready-to-use application services in a cluster, such as databases, message queues, and big data services, without having to manage their lifecycle manually. DKP handles the technical tasks of administration, scaling, backup, and updates, while the administrator manages services at the level of classes, limits, and architectural decisions.

## How it works

Each managed service adds its own Kubernetes resources and controllers to the cluster. A user describes the desired service state in a namespaced resource. The controller creates and updates the related objects: workload instances, Services, secrets, snapshots, and other resources supported by the specific managed service. The administrator manages cluster-wide resources that define allowed configurations and default values for user resources.

Use this section to:

- prepare the cluster to work with managed services;
- configure shared settings and limits;
- define available service classes and configurations;
- verify that the infrastructure is ready to run managed services.

For PostgreSQL service administration, see [Managed PostgreSQL](postgres.html).

Instructions for creating services, configuring instances, and connecting to them are provided in the [Usage](../../../user/managed-services/) section.
