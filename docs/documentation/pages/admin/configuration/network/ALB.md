---
title: "ALB in Deckhouse Kubernetes Platform"
permalink: en/admin/configuration/network/alb-overview.html
---

Deckhouse Kubernetes Platform (hereinafter referred to as DKP) supports application-level balancing of incoming traffic (ALB) by means of [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) (`ingress-nginx` module) and Istio (`istio` module).

Features of the ALB function in Deckhouse Kubernetes Platform:

- Automatic creation of load balancers. DKP automatically creates and configures ALBs based on Ingress resources.
- HTTP/HTTPS support. SSL/TLS termination and HTTP to HTTPS redirection is supported.
- Rule-based routing. You can route traffic based on path, host, or other request parameters.
- Certificate integration. Supports automatic certificate acquisition and renewal.
