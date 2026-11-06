---
title: Istio Ingress Gateway
description: The entry point of an application load balancer built on Istio. The administrator deploys it with [IngressIstioController](/modules/istio/cr.html#ingressistiocontroller), and application developers publish applications with Gateway and VirtualService objects.
lang: en
---

Publishing with Istio has two layers: the cluster administrator deploys the IngressIstioController and the related infrastructure, and developers create Gateway and VirtualService objects in their [namespace](glossary.html#namespace). This is one of the [ALB](glossary.html#alb) implementations in Deckhouse Platform, next to Ingress NGINX and the Gateway API with its [managed Gateway](glossary.html#alb-managed-gateway).

For details, see [Publishing applications using Istio](user/network/ingress/alb/istio.html).
