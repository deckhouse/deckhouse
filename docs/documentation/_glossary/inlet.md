---
title: Inlet
description: The way incoming traffic reaches an Ingress NGINX Controller. It is set in the inlet field of [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) and takes one of the values LoadBalancer, LoadBalancerWithProxyProtocol, HostPort or HostWithFailover.
lang: en
---

The `LoadBalancer` and `LoadBalancerWithProxyProtocol` inlets use a load balancer of the [cloud provider](glossary.html#cloud-provider) or the built-in [`metallb`](/modules/metallb/) module. The `HostPort` inlet exposes the [Ingress controller](glossary.html#ingress-controller) on node ports, for example behind an external load balancer. The `HostWithFailover` inlet is used on bare metal, where the controller runs on [frontend nodes](glossary.html#frontend-node).

For examples of each inlet, see [ALB with Ingress NGINX Controller](admin/configuration/network/ingress/alb/nginx.html).
