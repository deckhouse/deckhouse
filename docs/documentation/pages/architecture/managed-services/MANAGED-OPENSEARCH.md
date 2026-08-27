---
title: Managed-opensearch module
permalink: en/architecture/managed-services/managed-opensearch.html
search: managed-opensearch, opensearch
description: Architecture of the managed-opensearch module in Deckhouse Kubernetes Platform.
---

The [`managed-opensearch`](/modules/managed-opensearch/) module manages instances of [OpenSearch](https://github.com/opensearch-project/opensearch) in Deckhouse Kubernetes Platform (DKP). OpenSearch is an Open Source search and analytics engine designed for working with large volumes of data in real time.
The module provides:

* **Automatic Deployment**: Creates an OpenSearch instance using a simple YAML configuration.
* **Configuration Management**: Separate [OpensearchClass](/modules/managed-opensearch/cr.html#opensearchclass) resource for templating service creation and flexible validation of user parameters.
* **Class-oriented Management**: Supports sizing policies and CEL validations.
* **Long-term Storage**: Supports PersistentVolumeClaim parameters for data.
* **Status**: Displays the current status of the deployed OpenSearch instance.

For more details about module settings and usage examples, refer to the [module documentation](/modules/managed-opensearch/).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica of each Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`managed-opensearch`](/modules/managed-opensearch/) module and its interactions with other components of DKP are shown in the following diagram:

![Managed-opensearch module architecture](../../images/architecture/managed-services/c4-l2-managed-opensearch.svg)

## Module components

The module consists of the following components:

1. managed-opensearch-operator (Deployment): Kubernetes operator consisting of a single container manager that performs the following operations:

   * Reconciles [Opensearch](/modules/managed-opensearch/cr.html#opensearch) custom resources in all user namespaces. The Opensearch resource defines the settings of the OpenSearch instance.

   * Creates and manages StatefulSet, Service, Secret, and PersistentVolumeClaim resources related to the OpenSearch instance.

1. managed-opensearch-webhook (Deployment): A component consisting of a single container manager.

   It validates and mutates [Opensearch](/modules/managed-opensearch/cr.html#opensearch) custom resources using the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.

1. d8ms-osch-\<INSTANCE_NAME> (StatefulSet): A component consisting of a single opensearch container that launches and prepares the OpenSearch instance. It is created by the managed-opensearch-operator component.

## Module interactions

The module interacts with the kube-apiserver component:

* Manages Opensearch and [Certificate](https://cert-manager.io/docs/usage/certificate/) custom resources.
* Watches OpensearchClass custom resources.
* Manages StatefulSet, Service, Secret, and PersistentVolumeClaim resources.

The following external components interact with the module:

1. kube-apiserver: Processes requests to validate and mutate Opensearch custom resources.

1. User applications: Send requests to the OpenSearch instance.
