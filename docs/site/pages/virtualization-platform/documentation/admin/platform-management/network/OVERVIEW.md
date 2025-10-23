---
title: "Overview"
permalink: en/virtualization-platform/documentation/admin/platform-management/network/
---

Deckhouse Virtualization Platform (DVP) handles network configuration through modules and custom resources,
which simplifies and standardizes the setup process.

The platform supports:

- Management of incoming and outgoing traffic.
- Management of network policies and routing.
- DNS management within the cluster.

## Configuring and managing incoming traffic

[Incoming traffic](../network/ingress/) is managed using Ingress controllers.
These controllers route user requests to the appropriate applications and services based on rules defined in Ingress resources.
In DVP, traffic can be managed at both the network level (NLB – Network Load Balancer)
and the application level (ALB – Application Load Balancer).

## Managing outgoing traffic

DVP administrators can configure and manage [outgoing traffic](../network/egress/gateway.html)
to ensure correct processing and routing of all outgoing data.

Configuring outgoing traffic enables connection control and filtering,
which is especially important for secure integrations with external APIs and services.
This can be achieved by defining policies that allow or block specific traffic flows.

Outgoing traffic management is implemented using the Egress Gateway feature
(provided by the [`cni-cilium`](/modules/cni-cilium/) module).

## Internal network configuration

DVP provides a wide range of options for managing the [internal network](../network/internal/configuration.html).
Administrators can configure pod-to-pod and pod-to-node communication, as well as traffic encryption using various technologies and tools.

## Network policies

[Network policies](../network/policy/configuration.html) in DVP define rules that regulate traffic flow between pods, nodes, namespaces, and external systems.
Network policies ensure pod isolation, protect against internal cluster attacks,
and provide control over access to external services as well as incoming and outgoing connections.
