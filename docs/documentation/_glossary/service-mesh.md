---
title: Service mesh
description: An infrastructure layer that manages network communication between the services of an application. Encryption, access control, routing, request retries and metrics collection are configured centrally, without changing the code of the services.
lang: en
---

The services of an application exchange requests over the network, and without a service mesh each of them has to implement encryption, authorization, retries and timeouts on its own. A service mesh takes over these tasks: traffic between services passes through proxies that are configured from a single place. Usually the proxy is added to the application pod as a sidecar container. In the experimental ambient mode the proxy runs at the node level, and pods need no sidecars.

In Deckhouse Platform the service mesh is provided by the [`istio`](/modules/istio/) module. Within a single cluster, it is used, for example, for [canary deployment](glossary.html#canary-deployment) and for automatically cutting off a failing service (Circuit Breaker). It also forms the basis for [federation](glossary.html#federation) and [multicluster](glossary.html#multicluster).

For details, see [Inter-cluster cooperation](admin/configuration/network/alliance/).
