---
title: Inlet
description: The way incoming traffic reaches an application-level load balancer — Ingress NGINX Controller or a gateway of the [`alb`](/modules/alb/) module. It is set in the inlet field of [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller), [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) and [ALBInstance](/modules/alb/cr.html#albinstance) resources.
lang: en
---

For the [Ingress controller](glossary.html#ingress-controller), the inlet takes one of the values `LoadBalancer`, `LoadBalancerWithProxyProtocol`, `HostPort` or `HostWithFailover`. The `LoadBalancer` and `LoadBalancerWithProxyProtocol` inlets use a load balancer of the [cloud provider](glossary.html#cloud-provider) or the built-in [`metallb`](/modules/metallb/) module. The `HostPort` inlet exposes the controller on node ports, for example behind an external load balancer. The `HostWithFailover` inlet is used on bare metal, where the controller runs on [frontend nodes](glossary.html#frontend-node).

For gateways of the `alb` module, the inlet takes one of the values `LoadBalancer`, `HostPort` or `ClusterIP`. ClusterALBInstance supports `LoadBalancer` and `HostPort`, ALBInstance supports `LoadBalancer` and `ClusterIP`. The `ClusterIP` inlet accepts traffic through a service of the `ClusterIP` type, without an external entry point.

For examples of each inlet, see [ALB with Ingress NGINX Controller](admin/configuration/network/ingress/alb/nginx.html) and the [Inlets](admin/configuration/network/ingress/alb/alb-gateway-api.html#inlets) section of the "ALB with Kubernetes Gateway API" page.
