---
title: Managed-cassandra module
permalink: en/architecture/managed-services/managed-cassandra.html
search: managed-cassandra, cassandra
description: Architecture of the managed-cassandra module in Deckhouse Platform.
---

The [`managed-cassandra`](/modules/managed-cassandra/) module manages instances of the distributed, Open Source NoSQL database system [Apache Cassandra](https://github.com/apache/cassandra) in Deckhouse Platform (DP). It provides:

* **Automatic Deployment**: Creates a Cassandra instance using a simple YAML configuration.
* **Standalone**: Supports deployment of a standalone instance.
* **Configuration Management**: Separate [CassandraClass](/modules/managed-cassandra/cr.html#cassandraclass) resource for templating service creation and flexible validation of user parameters.
* **Status**: Displays the current status of the deployed Cassandra instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-cassandra/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-cassandra`](/modules/managed-cassandra/) module and its interactions with other components of DP are shown in the following diagram:

![Managed-cassandra module architecture](../../images/architecture/managed-services/c4-l2-managed-cassandra.svg)

## Module components

The module consists of the following components:

1. **Managed-cassandra-operator** (Deployment): Kubernetes operator consisting of a single container **manager** that performs the following operations:

   * Reconciles [Cassandra](/modules/managed-cassandra/cr.html#cassandra) custom resources in all user namespaces. The Cassandra and CassandraClass custom resources define the settings of the Cassandra instance.

   * Creates and manages StatefulSet, Service, Secret, and PersistentVolumeClaim resources related to the Cassandra instance.

   * Validates and mutates [Cassandra](/modules/managed-cassandra/cr.html#cassandra) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. **D8ms-cas-\<INSTANCE_NAME>** (StatefulSet): A component consisting of a single **cassandra** container that starts and runs the Cassandra instance. It is created by the managed-cassandra-operator component for each Cassandra custom resource.

## Module interactions

The module interacts with the **kube-apiserver** component:

* Manages Cassandra custom resources.
* Watches CassandraClass custom resources.
* Manages StatefulSet, Service, Secret, and PersistentVolumeClaim resources.

The following external components interact with the module:

1. **Kube-apiserver**: Validates and mutates Cassandra custom resources.

1. **User applications**: Send requests to the Cassandra instance.
