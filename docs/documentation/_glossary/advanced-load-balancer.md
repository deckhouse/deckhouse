---
title: Advanced load balancer
description: A load balancer of the [`service-with-healthchecks`](/modules/service-with-healthchecks/) module that runs active probes for individual TCP ports instead of relying on container readiness probes, so that each balancer serving the same pod works independently.
lang: en
---

The advanced load balancer is built on [ServiceWithHealthchecks](/modules/service-with-healthchecks/cr.html#servicewithhealthchecks) objects. It consists of a controller that runs on the [master nodes](glossary.html#master-node) and manages these objects, and agents on every node that run the probes and publish only verified addresses. The standard load balancer, a Kubernetes Service, remains available for workloads that need no separate probes.

For details, see [Intra-cluster communication](user/network/intra-cluster.html).
