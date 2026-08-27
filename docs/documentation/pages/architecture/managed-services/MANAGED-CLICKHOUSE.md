---
title: Managed-clickhouse module
permalink: en/architecture/managed-services/managed-clickhouse.html
search: managed-clickhouse, clickhouse
description: Architecture of the managed-clickhouse module in Deckhouse Kubernetes Platform.
---

The [`managed-clickhouse`](/modules/managed-clickhouse/) module manages instances of [ClickHouse](https://github.com/clickhouse/clickhouse) in Deckhouse Kubernetes Platform (DKP). ClickHouse is a high-performance Open Source columnar DBMS designed for online analytical processing (OLAP) of large volumes of data in real time.
The module provides:

* **Automatic Deployment**: Creates a ClickHouse instance using a simple YAML configuration.
* **Standalone**: Supports deployment of a standalone instance.
* **Configuration Management**: Separate [ClickhouseClass](/modules/managed-clickhouse/cr.html#clickhouseclass) resource for templating service creation and flexible validation of user parameters.
* **Status**: Displays the current status of the deployed ClickHouse instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-clickhouse/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-clickhouse`](/modules/managed-clickhouse/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-clickhouse module architecture](../../images/architecture/managed-services/c4-l2-managed-clickhouse.svg)

## Module components

The module consists of the following components:

1. managed-clickhouse-operator (Deployment): Kubernetes operator consisting of a single container manager that performs the following operations:

   * Reconciles [Clickhouse](/modules/managed-clickhouse/cr.html#clickhouse) custom resources in all user namespaces. The Clickhouse resource defines the settings of the ClickHouse instance.

   * Creates and manages StatefulSet, Service, Secret, ConfigMap, and PersistentVolumeClaim resources related to the ClickHouse instance.

   * Validates and mutates [Clickhouse](/modules/managed-clickhouse/cr.html#clickhouse) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. d8ms-ch-\<INSTANCE_NAME> (StatefulSet): A component consisting of a single clickhouse container that launches the ClickHouse instance.

## Module interactions

The module interacts with the kube-apiserver component:

* Manages Clickhouse and [Certificate](https://cert-manager.io/docs/usage/certificate/) custom resources.
* Watches ClickhouseClass custom resources.
* Manages StatefulSet, Service, Secret, ConfigMap, and PersistentVolumeClaim resources.

The following external components interact with the module:

1. kube-apiserver: Processes requests to validate and mutate Clickhouse custom resources.

1. User applications: Send requests to the ClickHouse instance.
