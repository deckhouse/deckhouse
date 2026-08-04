---
title: Managed-memcached module
permalink: en/architecture/managed-services/managed-memcached.html
search: managed-memcached, memcached
description: Architecture of the managed-memcached module in Deckhouse Kubernetes Platform.
---

The [`managed-memcached`](/modules/managed-memcached/) module simplifies the deployment and management of Memcached instances in Deckhouse Kubernetes Platform (DKP). It provides:

* **Automated Deployment**: Deploy Memcached instances with a simple YAML configuration.
* **High Availability**: Support for both standalone (Standalone) and group (Group) deployments.
* **Configuration Management**: Flexible configuration with validation and constraints through MemcachedClass.
* **Resource Management**: Automatic resource allocation and scaling.
* **Monitoring**: Built-in status tracking and health monitoring.
* **Security**: Distroless images and proper RBAC configuration.
* **Validation**: CEL rules for configuration validation.

For more details about module settings and usage examples, refer to [the module documentation](/modules/managed-memcached/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica per Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-memcached`](/modules/managed-memcached/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-memcached module architecture](../../images/architecture/managed-services/c4-l2-managed-memcached.ru.png)

## Module components

The module consists of the following components:

1. **Managed-memcached-operator**: Kubernetes operator consisting of a single container **manager** that performs the following operations:

   * It reconcyles [Memcached](/modules/managed-memcached/stable/cr.html#memcached) custom resources in all user namespaces. The Memcached resource defines the settings of the Memcached instance, including the topology and the deployment type.

   * It performs Memcached and MemcachedClass custom resources validation, as well as Memcached custom resources mutation using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. **d8ms-mc-\<instance name>** (StatefulSet): One or more Memcached instances, depending on [the deployment type](/modules/managed-memcached/stable/user_guide.html#standalone-vs-group). They are created by the managed-memcached-operator component.

   It consists of a single container:

   * **memcached**: It is an [open-source project](https://github.com/memcached/memcached.git).

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**: Manages Memcached custom resources.

The following external components interact with the module:

1. **Kube-apiserver**: Validates Memcached and MemcachedClass custom resources, mutates Memcached custom resources.

1. **Prometheus-main**: Collects Memcached instances metrics.

1. **User applications**: Sends requests to the Memcached instances.
