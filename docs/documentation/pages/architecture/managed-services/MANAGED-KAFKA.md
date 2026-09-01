---
title: Managed-kafka module
permalink: en/architecture/managed-services/managed-kafka.html
search: managed-kafka, kafka
description: Architecture of the managed-kafka module in Deckhouse Kubernetes Platform.
---

The [`managed-kafka`](/modules/managed-kafka/) module manages [Apache Kafka](https://github.com/apache/kafka) instances in Deckhouse Kubernetes Platform (DKP). Apache Kafka is a distributed Open Source data streaming platform and message broker.
The module provides:

* **Automatic Deployment**: Creates a Kafka instance using a simple YAML configuration.
* **Configuration Management**: Separate [KafkaClass](/modules/managed-kafka/cr.html#kafkaclass) resource for templating service creation and flexible validation of user parameters.
* **Long-term Storage**: Supports PersistentVolumeClaim parameters for data.
* **Status**: Displays the current status of the deployed Kafka instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-kafka/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-kafka`](/modules/managed-kafka/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-kafka module architecture](../../images/architecture/managed-services/c4-l2-managed-kafka.svg)

## Module components

The module consists of the following components:

1. managed-kafka-operator (Deployment): Kubernetes operator consisting of a single container manager that performs the following operations:

   * Reconciles [Kafka](/modules/managed-kafka/cr.html#kafka) custom resources in all user namespaces. The Kafka resource defines the settings of the Kafka instance.

   * Creates and manages StatefulSet, Service, Secret, and PersistentVolumeClaim resources related to the Kafka instance.

1. managed-kafka-webhook (Deployment): A component consisting of a single container manager.

   It validates and mutates [Kafka](/modules/managed-kafka/cr.html#kafka) and [KafkaClass](/modules/managed-kafka/cr.html#kafkaclass) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. d8ms-kfk-\<INSTANCE_NAME> (StatefulSet): A component consisting of a single kafka container that starts and prepares the Kafka instance. It is created by the managed-kafka-operator component.

## Module interactions

The module interacts with the kube-apiserver component:

* Manages Kafka and [Certificate](https://cert-manager.io/docs/usage/certificate/) custom resources.
* Watches KafkaClass custom resources.
* Manages StatefulSet, Service, Secret, and PersistentVolumeClaim resources.

The following external components interact with the module:

1. kube-apiserver: Processes requests to validate and mutate Kafka and KafkaClass custom resources.

1. User applications: Send requests to the Kafka instance.
