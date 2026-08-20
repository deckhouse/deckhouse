---
title: Managed-valkey module
permalink: en/architecture/managed-services/managed-valkey.html
search: managed-valkey, valkey
description: Architecture of the managed-valkey module in Deckhouse Kubernetes Platform.
---

The [`managed-valkey`](/modules/managed-valkey/) module manages [Valkey](https://github.com/valkey-io/valkey) clusters (a Redis-compatible in-memory data store) in Deckhouse Kubernetes Platform (DKP). It provides:

* **Automatic Deployment**: Deploys a Valkey instance using a simple YAML configuration.
* **Standalone**: Supports deployment of a standalone instance.
* **Persistent Storage**: Allows configuring different data storage options: `AOF`, `RDB`, `AOF+RDB`.
* **Configuration Management**: Separate [ValkeyClass](/modules/managed-valkey/cr.html#valkeyclass) resource for templating approach to service creation with the ability to flexibly validate user configs.
* **Status**: Informative set of states for tracking the deployed Valkey.

For more details about module settings and usage examples, refer to [the module documentation](/modules/managed-valkey/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica per Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-valkey`](/modules/managed-valkey/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-valkey module architecture](../../images/architecture/managed-services/c4-l2-managed-valkey.svg)

## Module components

The module consists of the following components:

1. **Managed-valkey-operator** (Deployment): Kubernetes operator consisting of a single container **manager** that performs the following operations:

   * It reconciles [Valkey](/modules/managed-valkey/cr.html#valkey) custom resources in all user namespaces. The Valkey resource defines the settings of the Valkey instance.

   * It creates and manages StatefulSet, Secret, ConfigMap, and PersistentVolumeClaim resources related to the Valkey instance.

1. **Managed-valkey-webhook** (Deployment): Component consisting of a single container **manager**.

   Managed-valkey-webhook validates and mutates [Valkey](/modules/managed-valkey/cr.html#valkey) custom resources, and mutates [ValkeyClass](/modules/managed-valkey/cr.html#valkeyclass) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. **D8ms-valkey-\<INSTANCE_NAME>** (StatefulSet): Component that starts and prepares the Valkey instance. It is created by the managed-valkey-operator component.

   It consists of two containers:

   * **valkey**: It is an [open-source project](https://github.com/valkey-io/valkey).
   * **agent**: Sidecar container that configures the main container according to the parameters in the Valkey resource.

## Module interactions

The module interacts with the **kube-apiserver** component:

* It manages Valkey, ValkeyClass, and [Certificate](https://cert-manager.io/docs/usage/certificate/) custom resources.
* It manages StatefulSet, Secret, ConfigMap, and PersistentVolumeClaim resources.

The following external components interact with the module:

1. **Kube-apiserver**: Sends validation and mutation requests for Valkey custom resources.

1. **Prometheus-main**: Collects metrics of the managed-valkey-operator and managed-valkey-webhook components.

1. **User applications**: Sends requests to the Valkey instance.
