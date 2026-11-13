---
title: "The cni-cilium module: usage"
description: "Reaching pods of other clusters and global services in clusters connected by peering."
---

## Cluster peering

{% alert level="warning" %}
Experimental, available in the Ultimate/EE editions only.
{% endalert %}

When the administrator has meshed the cluster with other clusters (see [Cluster peering](advanced_usage.html#cluster-peering) in advanced usage), a pod reaches pods of other clusters by IP address, and a service can use backends of other clusters.

### Pods of other clusters

A pod reaches a pod of another cluster by its IP address, like a pod of its own cluster. There is no NAT: the receiving pod sees the client pod's address.

Pods and services of other clusters have no DNS names. To reach an application of another cluster by name, make its service global.

### Global services

A global service uses its own backends and the backends of the service with the same name in the same namespace in the neighbour clusters. Clients reach it by its name and address in their own cluster.

A service is configured with annotations:

| Annotation | Value |
|---|---|
| `network.deckhouse.io/global-service-mode` | `Local`: the service is local (same as no annotation).<br>`ImportOnly`: uses the neighbours' backends, does not offer its own.<br>`ImportAndExport`: uses the neighbours' backends and offers its own |
| `network.deckhouse.io/global-service-traffic-distribution` | `Any` (default): backends of all clusters.<br>`PreferLocalCluster`: own backends while any is ready.<br>`PreferRemoteCluster`: the neighbours' backends while any is ready |

An unknown mode means `Local`, an unknown distribution means `Any`. The distribution applies only in the `ImportOnly` and `ImportAndExport` modes. Changes apply without restarting pods; open connections are not moved.

The service name, namespace and ports (with names) must match in every cluster.

A headless service (`clusterIP: None`) can be global too: its DNS name returns the addresses of the pods of this and the neighbour clusters. The traffic distribution does not apply to it. Name the StatefulSet pods behind such a service differently in each cluster, for example with the cluster name: equal pod names from different clusters conflict in DNS.

Common tasks:

| Task | What to do |
|---|---|
| Balance over the replicas of all clusters | `global-service-mode: ImportAndExport` in every cluster |
| Use own replicas, the neighbours as a fallback | Add `global-service-traffic-distribution: PreferLocalCluster` |
| Use an application that does not run in your cluster | In your cluster, a service with `ImportOnly`; in the cluster running the application, with `ImportAndExport` |
| Drain a cluster without stopping its replicas | In that cluster, `ImportOnly` and `PreferRemoteCluster`. After maintenance, set `ImportAndExport` back and remove the distribution |

Example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api
  namespace: shop
  annotations:
    network.deckhouse.io/global-service-mode: ImportAndExport
    network.deckhouse.io/global-service-traffic-distribution: PreferLocalCluster
spec:
  selector:
    app: api
  ports:
  - name: http
    port: 80
    targetPort: 8080
```

When the mesh with a neighbour is torn down, its backends are removed from global services and connections to its pods break. A temporary loss of connectivity between the clusters does not tear the mesh down.
