---
title: Ingress controller
description: A controller that routes user requests to applications and services by the rules defined in Ingress resources.
lang: en
---

In Deckhouse Platform, the Ingress controller is Ingress NGINX Controller, managed by the [`ingress-nginx`](/modules/ingress-nginx/) module through [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) resources. Several Ingress NGINX Controllers can run in one cluster at the same time, one of them is the primary one. Kubernetes Gateway API (the [`alb`](/modules/alb/) module) and [Istio Ingress Gateway](glossary.html#istio-ingress-gateway) also balance traffic at the application level, but they work not with Ingress resources but with Gateway API routes and Gateway and VirtualService resources. All three solutions are implementations of [ALB](glossary.html#alb).

Ingress controllers are usually placed on [frontend nodes](glossary.html#frontend-node). The way traffic reaches a controller is defined by its [inlet](glossary.html#inlet). For details, see [Incoming traffic balancing](admin/configuration/network/ingress/).
