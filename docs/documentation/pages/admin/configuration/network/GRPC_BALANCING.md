---
title: "gRPC balancing"
permalink: en/admin/network/grpc-balancing.html
---

In Deckhouse Kubernetes Platform, gRPC balancing is implemented by Istio tools (module [`istio`](../#)).

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#mutual-tls -->

**Caution!** Assign a name with the `grpc` prefix or value to the port in the corresponding Service to make gRPC service balancing start automatically.
