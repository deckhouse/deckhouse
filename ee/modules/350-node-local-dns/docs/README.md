---
title: "The node-local-dns module"
---

## Module Description

This module deploys a caching DNS server on each cluster node and exports data to `Prometheus` for easy analysis of the cluster's DNS performance using the Grafana [board](#grafana-dashboard).

The module runs the standard `CoreDNS` server (as part of a DaemonSet) on all nodes supplemented by the algorithm for configuring network and iptables rules.

### Purpose

The regular DNS usage in Kubernetes poses some problems that may lead to unnecessary degradation of the service's performance:
- No request caching (in Linux, there is no caching right out-of-the-box; thus, Pods also lack it out-of-the-box).
- All the container-originated DNS requests result in a network request to the cluster DNS. Thus, even requests to resources within the same node result in network requests.
- The Pod request is first resolved in the cluster DNS zones and only then channeled to external DNS servers. For example, the request to `ya.ru` is first resolved in cluster zones such as `cluster.local`, `svc.cluster.local`, `<namespace>.svc.cluster.local`, receives negative responses, and gets appropriately resolved only on the "second" attempt.

Thus, even slight network delays may result in significant service performance degradation due to above problems.

One solution is to deploy a DNS server on each node (which is what this module does).

Outside requests (non-cached ones) are also first resolved in internal zones. Under high load (when a large number of requests are made to the same records simultaneously, as is often the case), such behavior can significantly improve the performance of DNS resolving.

## Grafana dashboard

The `Kubernetes / DNS (node local)` dashboard displays:
- General graphs that allow you to evaluate the overall DNS performance.
- Graphs grouped by the node that let you analyze node data in more details if the general charts suggest that this node is unhealthy.
- Upstream-related graphs that allow you to evaluate the performance of the cluster DNS as well as the node servers specified in `/etc/resolv.conf`.

## How does it work?

The module performs the following settings on each node:
- Configures an interface with the ClusterIP IP address of the `kube-dns` service.
- Starts a caching CoreDNS server that listens on this address.
- Adds a sophisticated iptables rule that uses a socket if it is open and ordinary Kubernetes magic for clusterIP if it is not.

And here is the rule itself that allows for an easy fallback:

```bash
-A PREROUTING -d <kube-dns IP address> -m socket --nowildcard -j NOTRACK
```

### Aspects of CoreDNS configuration

Main features of the CoreDNS config:
- caching of all requests;
- forwarding all DNS requests to ClusterIP of the cluster DNS.
