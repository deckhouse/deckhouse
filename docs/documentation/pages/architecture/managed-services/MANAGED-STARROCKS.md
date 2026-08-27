---
title: Managed-starrocks module
permalink: en/architecture/managed-services/managed-starrocks.html
search: managed-starrocks, starrocks
description: Architecture of the managed-starrocks module in Deckhouse Kubernetes Platform.
---

The [`managed-starrocks`](/modules/managed-starrocks/) module manages [StarRocks](https://github.com/starrocks/starrocks) instances in Deckhouse Kubernetes Platform (DKP). StarRocks is a high-performance analytical DBMS (OLAP) for real-time analytics, data warehouses, and BI workloads.

The module provides:

* **Automatic Deployment**: Creates a StarRocks instance using a simple YAML configuration.
* **Configuration Management**: Separate [StarrocksClass](/modules/managed-starrocks/cr.html#starrocksclass) resource for templating service creation and flexible validation of user parameters.
* **Analytics Tuning**: Supports data load parameters (`streamingLoadMaxMb`, `loadProcessMaxMemoryLimitPercent`) and catalog management (`catalogTrashExpireSecond`).
* **Status**: Displays the current status of the deployed StarRocks instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-starrocks/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-starrocks`](/modules/managed-starrocks/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-starrocks module architecture](../../images/architecture/managed-services/c4-l2-managed-starrocks.svg)

## Module components

The module consists of the following components:

1. managed-starrocks-operator (Deployment): Kubernetes operator consisting of a single container manager that performs the following operations:

   * Reconciles [Starrocks](/modules/managed-starrocks/cr.html#starrocks) custom resources in all user namespaces. The Starrocks resource defines the settings of the StarRocks instance.

   * Creates and manages StatefulSet, Service, Secret, ConfigMap, and PersistentVolumeClaim resources related to the StarRocks instance.

1. managed-starrocks-webhook (Deployment): A component consisting of a single container manager.

   It validates and mutates [Starrocks](/modules/managed-starrocks/cr.html#starrocks) custom resources, and mutates [StarrocksClass](/modules/managed-starrocks/cr.html#starrocksclass) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. d8ms-sr-\<INSTANCE_NAME> (StatefulSet): A component that starts and prepares the StarRocks instance. It is created by the managed-starrocks-operator component.

   It consists of the following containers:

   * **agent**: Sidecar container that monitors and updates the TLS certificate status, and configures the instance.
   * **starrocks**: Main container.

## Module interactions

The module interacts with the kube-apiserver component:

* Manages Starrocks and [Certificate](https://cert-manager.io/docs/usage/certificate/) custom resources.
* Watches StarrocksClass custom resources.
* Manages StatefulSet, Service, Secret, ConfigMap, and PersistentVolumeClaim resources.

The following external components interact with the module:

1. kube-apiserver: Processes requests to validate and mutate Starrocks and StarrocksClass custom resources.

1. User applications: Send requests to the StarRocks instance.
