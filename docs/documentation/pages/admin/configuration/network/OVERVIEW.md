---
title: "Overview"
permalink: en/admin/configuration/network/
description: "Configure networking in Deckhouse Kubernetes Platform with CNI, ingress, egress, load balancing, and network policies. Complete network configuration and management guide."
---

Unlike basic Kubernetes installations,
Deckhouse Kubernetes Platform (DKP) handles network configuration through modules and custom resources,
which simplifies and standardizes the setup process.

The platform supports:

- Management of incoming and outgoing traffic.
- Configuration of inter-cluster communication.
- Management of network policies and routing.
- DNS management within the Kubernetes cluster.

## Configuring and managing incoming traffic

[Incoming traffic](../network/ingress/) is managed using Ingress controllers.
These controllers route user requests to the appropriate applications and services based on rules defined in Ingress resources.
In DKP, traffic can be managed at both the network level (NLB – Network Load Balancer)
and the application level (ALB – Application Load Balancer).

## Managing outgoing traffic

DKP administrators can configure and manage [outgoing traffic](../network/egress/gateway.html)
to ensure correct processing and routing of all outgoing data.

Configuring outgoing traffic enables connection control and filtering,
which is especially important for secure integrations with external APIs and services.
This can be achieved by defining policies that allow or block specific traffic flows.

Outgoing traffic management is implemented using the Egress Gateway feature
(provided by the [`cni-cilium`](/modules/cni-cilium/) module) as well as Istio tools (via the [`istio`](/modules/istio/) module).

## Internal network configuration

DKP provides a wide range of options for managing the [internal network](../network/internal/configuration.html).
Administrators can configure pod-to-pod and pod-to-node communication, as well as traffic encryption using various technologies and tools.

## Configuring inter-cluster communication

DKP supports two approaches to organizing communication between independent and codependent clusters:

- [Multicluster](../network/alliance/multicluster.html) — combines clusters to share resources,
  balance loads, and improve fault tolerance.
  Ideal for distributed teams and infrastructure spread across regions.
- [Federation](../network/alliance/federation.html) — centralized management of multiple independent clusters
  with unified policies and synchronized configurations.
  Effective for large organizations and complex systems.

For more information on configuring inter-cluster communication, refer to [Inter-cluster communication](../network/alliance/).

## Network policies

Network policies in DKP define rules that regulate traffic flow between pods, nodes, namespaces, and external systems.
Network policies ensure pod isolation, protect against internal cluster attacks,
and provide control over access to external services as well as incoming and outgoing connections.
