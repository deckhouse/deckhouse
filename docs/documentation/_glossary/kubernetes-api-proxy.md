---
title: kubernetes-api-proxy
description: An NGINX-based proxy on every cluster node that serves requests to the Kubernetes API sent to localhost and forwards them to the master nodes, providing high availability and load balancing of the API.
lang: en
---

[bashible](glossary.html#bashible) installs and configures the proxy and keeps its list of upstream [master nodes](glossary.html#master-node) up to date. On master nodes it runs as a [static pod](glossary.html#static-pod): by default it proxies requests to the local kube-apiserver and, if that is unavailable, queries the remaining instances in turn. Requests from kubelet to `localhost` go through it.

For details, see [Control plane management](architecture/kubernetes-and-scheduling/control-plane-management.html).
