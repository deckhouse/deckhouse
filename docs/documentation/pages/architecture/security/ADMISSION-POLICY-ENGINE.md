---
title: Admission-policy-engine module
permalink: en/architecture/security/admission-policy-engine.html
search: admission-policy-engine, pod security, gatekeeper
description: Architecture of the admission-policy-engine module in Deckhouse Kubernetes Platform.
---

The [`admission-policy-engine`](/modules/admission-policy-engine/) module enforces security policies and operational restrictions in a Kubernetes cluster. Policies are applied based on [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) checks and rules from the following custom resources:

- OperationPolicy: Describes the operational policy of the cluster.
- SecurityPolicy: Describes the security policy of the cluster.
- SecurityPolicyException: Describes exceptions to the cluster security policy.

{% alert level="info" %}
The [`deckhouse`](/modules/deckhouse/) module processes the OperationPolicy and SecurityPolicy resources. The Deckhouse controller of the `deckhouse` module uses [addon-operator](https://flant.github.io/addon-operator/OVERVIEW.html) and [module hooks](../module-development/structure/#hooks) to create custom resources for [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) based on OperationPolicy and SecurityPolicy. Gatekeeper uses the resulting custom resources to validate newly created or updated Kubernetes resources.

For details on the hooks concept, refer to the [addon-operator documentation](https://flant.github.io/addon-operator/OVERVIEW.html).
{% endalert %}

For a detailed description of the module, refer to [the corresponding documentation section](/modules/admission-policy-engine/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`admission-policy-engine`](/modules/admission-policy-engine/) module and its interaction with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Admission-policy-engine architecture](../../../images/architecture/security/c4-l2-admission-policy-engine.png)

## Module components

The module consists of the following components:

1. **Gatekeeper-controller-manager**: A [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) controller that performs the following operations:

   * Manages [Gatekeeper custom resources](https://github.com/open-policy-agent/gatekeeper/tree/master/charts/gatekeeper/crds).
   * Validates Kubernetes resources specified in custom resources from the `constraints.gatekeeper.sh/*` API group.
   * Mutates Kubernetes resources specified in the [AssignMetadata](/modules/admission-policy-engine/gatekeeper-cr.html#assignmetadata), [Assign](/modules/admission-policy-engine/gatekeeper-cr.html#assign), [ModifySet](/modules/admission-policy-engine/gatekeeper-cr.html#modifyset), and [AssignImage](/modules/admission-policy-engine/gatekeeper-cr.html#assignimage) custom resources.

   Security rules are defined using the ConstraintTemplate custom resource and custom resources from the `constraints.gatekeeper.sh/*` API group. A ConstraintTemplate defines new policy types, based on which specific security policies are created to validate resources.

   It consists of the following containers:

   * **manager**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to controller metrics.

1. **Gatekeeper-audit**: Implements periodic checks of existing Kubernetes resources for compliance with security policies.

   It consists of the following containers:

   * **manager**: Main container.
   * **constraint-exporter**: Sidecar container that exposes additional metrics for the `constraints.gatekeeper.sh/*` and `mutations.gatekeeper.sh/*` custom resources.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to metrics from `manager` and `constraint-exporter`.

1. **Ratify**: An optional component consisting of a single [**ratify**](https://ratify.dev/docs/what-is-ratify) container. It provides a [Gatekeeper provider](https://open-policy-agent.github.io/gatekeeper/website/docs/externaldata) implementation for validating metadata of used artifacts. In DKP, this provider is used to verify container image signatures and is available in the SE+, EE, CSE Lite, and CSE Pro editions.

{% alert level="info" %}
   Gatekeeper uses the Provider custom resource to extend resource validation capabilities in Kubernetes. The Provider resource describes the service endpoint to which Gatekeeper sends requests during ValidationWebhook execution. Some DKP modules, such as [`operator-trivy`](/modules/operator-trivy), can create Provider custom resources and thereby extend the verification capabilities.
{% endalert %}

## Module interactions

The module interacts with the following components:

* **Kube-apiserver**:

  * Monitors Kubernetes resources specified in custom resources of the `constraints.gatekeeper.sh/*` and `mutations.gatekeeper.sh/*` API groups.
  * Manages ConstraintTemplate, Assign, AssignImage, AssignMetadata, ModifySet, as well as resources from the `constraints.gatekeeper.sh/*` and `config.ratify.deislabs.io/*` API groups.

The following external components interact with the module:

1. **Kube-apiserver**: Validates Kubernetes resources and checks their compliance with the defined security rules.

1. **Prometheus-main**: Collects module metrics.
