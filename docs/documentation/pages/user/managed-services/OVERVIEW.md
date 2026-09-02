---
title: "Overview"
permalink: en/user/managed-services/
description: "Using managed services in Deckhouse Kubernetes Platform"
---

Managed services allow users to create and use ready-to-run application services by using Kubernetes custom resources.

## How it works

A user creates a custom resource of a managed service in their namespace, for example Postgres, and describes the desired configuration in it: resources, deployment mode, topology, users, and other parameters. The resource references a managed service class (for example, PostgresClass) prepared by the cluster administrator — the class defines the allowed parameters and limits.

The operator watches these resources and creates or updates the managed service's running instances according to the specified configuration. The resource status reflects the current state of the managed service.

Use this section to:

- create a managed service in your namespace;
- configure managed service instance parameters;
- connect to the created managed service;
- perform basic maintenance operations;
- check the resource status and configuration result.

For PostgreSQL creation and usage, see [Managed PostgreSQL](postgres.html).

Details about infrastructure preparation, managed service class configuration, and other administrative settings are provided in the [Administration](../../admin/configuration/managed-services/) section.
