---
title: ALB (application load balancing)
description: Load balancing at the application level. HTTP(S) headers, paths and domains are analysed, SSL termination and content-based routing are supported.
lang: en
---

Deckhouse Platform provides three ALB implementations: Ingress NGINX Controller (the [`ingress-nginx`](/modules/ingress-nginx/) module), Kubernetes Gateway API (the [`alb`](/modules/alb/) module) and Istio (the [`istio`](/modules/istio/) module). Balancing at the network level is [NLB](glossary.html#nlb).

For the criteria of choosing an implementation, see [Incoming traffic balancing](admin/configuration/network/ingress/).
