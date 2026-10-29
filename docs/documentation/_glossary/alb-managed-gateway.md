---
title: Managed Gateway
description: The Gateway object that the [`alb`](/modules/alb/) module creates from a [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) or an [ALBInstance](/modules/alb/cr.html#albinstance). Applications are published through it with ListenerSet and route resources of the Kubernetes Gateway API.
lang: en
---

A cluster-wide gateway is created by the cluster administrator from a ClusterALBInstance, usually in the `d8-alb` [namespace](glossary.html#namespace). A dedicated gateway in the application namespace is created from an ALBInstance. The name of the managed Gateway is published in the `status.gateway` field of the instance. The namespace administrator attaches ListenerSets with hostnames, TLS and ports, and developers attach HTTPRoute and other routes. The listeners `d8-http` and `d8-https` serve service tasks such as availability checks and are not meant for application routes.

For details, see [Publishing applications using the Kubernetes Gateway API](user/network/ingress/alb/gateway-api.html).
