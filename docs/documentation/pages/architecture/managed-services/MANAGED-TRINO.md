---
title: Managed-trino module
permalink: en/architecture/managed-services/managed-trino.html
search: managed-trino, trino
description: Architecture of the managed-trino module in Deckhouse Kubernetes Platform.
---

The [`managed-trino`](/modules/managed-trino/) module manages [Trino](https://github.com/trinodb/trino) instances in Deckhouse Kubernetes Platform (DKP). Trino is a fast Open Source distributed SQL engine built for big data analytics.
The module provides:

* **Automatic Deployment**: Creates a Trino instance using a simple YAML configuration.
* **Standalone**: Supports deployment of a standalone instance.
* **External Connections**: Supports working with the [Hive Metastore](https://github.com/apache/hive) metadata store, as well as with S3 storage.
* **Configuration Management**: Separate [TrinoClass](/modules/managed-trino/cr.html#trinoclass) resource for templating service creation and flexible validation of user parameters.
* **Status**: Displays the current status of the deployed Trino instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-trino/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-trino`](/modules/managed-trino/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-trino module architecture](../../images/architecture/managed-services/c4-l2-managed-trino.svg)

## Module components

The module consists of the following components:

1. managed-trino-operator (Deployment): Kubernetes operator consisting of a single container manager that performs the following operations:

   * Reconciles [Trino](/modules/managed-trino/cr.html#trino) custom resources in all user namespaces. The Trino resource defines the settings of the Trino instance.

   * Creates and manages Deployment, Service, Secret, and ConfigMap resources related to the Trino instance.

   * Validates and mutates [Trino](/modules/managed-trino/cr.html#trino) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. d8ms-trn-\<INSTANCE_NAME> (Deployment): A component consisting of a single trino container that launches the Trino instance.

## Module interactions

The module interacts with the following components:

1. Hive Metastore instance: Processes data in the metadata store.

1. S3 storage: Processes data in the object storage.

1. kube-apiserver:

   * Manages Trino custom resources.
   * Watches TrinoClass custom resources.
   * Manages Deployment, Service, Secret, and ConfigMap resources.

The following external components interact with the module:

1. kube-apiserver: Processes requests to validate and mutate Trino custom resources.

1. User applications: Send requests to the Trino instance.
