---
title: "The node-local-dns module"
---

## Module Description

This module deploys a caching DNS server on each cluster node and exports data to `Prometheus` for easy analysis of the cluster's DNS performance using the Grafana [board](#grafana-dashboard).

The module runs the standard `CoreDNS` server (as part of a DaemonSet) on all nodes supplemented by the algorithm for configuring network and iptables rules.

### Purpose

The regular DNS usage in Kubernetes poses some problems that may lead to unnecessary degradation of the service's performance:

* No request caching (in Linux, there is no caching right out-of-the-box; thus, Pods also lack it out-of-the-box).

* When the container makes a DNS request, it accesses the cluster DNS. If the request concerns resources on the same node, the network request is still executed.

* The Pod request is first resolved in the cluster DNS zones and only then channeled to external DNS servers. For example, the request to `ya.ru` is first resolved in cluster zones such as `cluster.local`, `svc.cluster.local`, `<namespace>.svc.cluster.local`, receives negative responses, and gets appropriately resolved only on the "second" attempt.

Due to the above problems, the quality of service may degrade in case of network delays. One solution is to install a DNS server on each node. The described `node-local-dns` module is designed for this.

External queries will also try to use internal zones to resolve names only if those names are not in the cache. This can significantly improve DNS name resolution, especially when the server is under heavy load (when there are many requests to resolve the same records per second).

## Grafana dashboard

The `Kubernetes / DNS (node local)` dashboard displays:

* General graphs that allow you to evaluate the overall DNS performance.

* Graphs grouped by the node that let you analyze node data in more details if the general charts suggest that this node is unhealthy.

* Upstream-related graphs that allow you to evaluate the performance of the cluster DNS as well as the node servers specified in `/etc/resolv.conf`.

## How does it work?

The module performs the following settings on each node:

* Configures an interface with the ClusterIP IP address of the `kube-dns` service.

* Starts a caching CoreDNS server that listens on this address.

* If the socket is closed, traffic is routed to the cluster IP address. If the socket is open, traffic is redirected to it. This is achieved by adding a special rule to iptables.

The rule for a trouble-free fallback is presented below:

```bash
-A PREROUTING -d <kube-dns IP address> -m socket --nowildcard -j NOTRACK
```

### Aspects of CoreDNS configuration

Main features of the CoreDNS config:

* caching of all requests;

* forwarding all DNS requests to ClusterIP of the cluster DNS.
