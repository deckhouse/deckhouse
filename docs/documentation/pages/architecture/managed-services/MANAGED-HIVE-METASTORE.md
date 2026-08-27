---
title: Managed-hive-metastore module
permalink: en/architecture/managed-services/managed-hive-metastore.html
search: managed-hive-metastore, hive-metastore
description: Architecture of the managed-hive-metastore module in Deckhouse Kubernetes Platform.
---

The [`managed-hive-metastore`](/modules/managed-hive-metastore/) module manages instances of [Hive Metastore (HMS)](https://github.com/apache/hive) in Deckhouse Kubernetes Platform (DKP). HMS is the centralized metadata store in the big data ecosystem.
The module provides:

* **Automatic Deployment**: Creates an HMS instance using a simple YAML configuration.
* **Standalone**: Supports deployment of a standalone instance.
* **Configuration Management**: Separate [HiveMetastoreClass](/modules/managed-hive-metastore/cr.html#hive-metastoreclass) resource for templating service creation and flexible validation of user parameters.
* **Status**: Displays the current status of the deployed HMS instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-hive-metastore/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-hive-metastore`](/modules/managed-hive-metastore/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-hive-metastore module architecture](../../images/architecture/managed-services/c4-l2-managed-hive-metastore.svg)

## Module components

The module consists of the following components:

1. managed-hive-metastore-operator (Deployment): Kubernetes operator consisting of a single container manager that performs the following operations:

   * Reconciles [HiveMetastore](/modules/managed-hive-metastore/cr.html#hive-metastore) custom resources in all user namespaces. The HiveMetastore resource defines the settings of the HMS instance.

   * Creates and manages Deployment, Service, and Secret resources related to the HMS instance.

   * Validates and mutates [HiveMetastore](/modules/managed-hive-metastore/cr.html#hive-metastore) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. d8ms-hms-\<INSTANCE_NAME> (Deployment): A component that launches the HMS instance.

   It consists of the following containers:

   * **truststore-prepare**: Init container that prepares the truststore.
   * **agent**: Sidecar container that monitors and updates the TLS certificate status.
   * **hivemetastore**: Main container.

## Module interactions

The module interacts with the following components:

1. PostgreSQL instance: Processes metadata on the database server.

1. S3 storage: Processes data in the object storage.

1. kube-apiserver:

   * Manages HiveMetastore and [Certificate](https://cert-manager.io/docs/usage/certificate/) custom resources.
   * Watches HiveMetastoreClass custom resources.
   * Manages Deployment, Service, and Secret resources.

The following external components interact with the module:

1. kube-apiserver: Processes requests to validate and mutate HiveMetastore custom resources.

1. User applications: Send requests to the HMS instance.
