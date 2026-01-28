---
title: "Overview"
permalink: en/virtualization-platform/documentation/admin/platform-management/network/ingress/
---

This section describes the approaches to balancing incoming traffic in Deckhouse Virtualization Platform (DVP):

- NLB (Network Load Balancer) — operates at the network level, routing traffic based on IP addresses
  and ports without inspecting request contents.
- ALB (Application Load Balancer) — operates at the application level, analyzing HTTP(S) headers, paths, and domains.
  It supports SSL termination and content-based routing.

## Network-level load balancing (NLB)

NLB-based load balancing can be implemented in two ways:

- Using an external load balancer provided by a cloud provider.
- Using the built-in MetalLB balancer, which works in both cloud and bare-metal clusters.

## Application-level load balancing (ALB)

For application-level traffic balancing DVP uses [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) (via the [`ingress-nginx`](/modules/ingress-nginx/) module).
