---
title: Ingress controller
description: A controller that routes user requests to applications and services by the rules defined in Ingress resources.
lang: en
---

In Deckhouse Platform, Ingress controllers are managed by modules and custom resources. The available implementations are Ingress NGINX Controller (the [`ingress-nginx`](/modules/ingress-nginx/) module), Kubernetes Gateway API (the [`alb`](/modules/alb/) module) and Istio (the [`istio`](/modules/istio/) module). Several Ingress NGINX Controllers can run in one cluster at the same time, one of them is the primary one.

Ingress controllers are usually placed on [frontend nodes](glossary.html#frontend-node). The way traffic reaches a controller is defined by its [inlet](glossary.html#inlet). For details, see [Incoming traffic balancing](admin/configuration/network/ingress/).
