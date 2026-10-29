---
title: NLB (network load balancing)
description: Load balancing at the network level. Traffic is routed by IP addresses and ports without inspecting the contents of requests.
lang: en
---

In Deckhouse Platform, NLB is implemented either by an external load balancer of the [cloud provider](glossary.html#cloud-provider) or by the built-in [`metallb`](/modules/metallb/) module, which works both in cloud and in bare-metal clusters. Balancing at the application level is [ALB](glossary.html#alb).

For details, see [Incoming traffic balancing](admin/configuration/network/ingress/).
