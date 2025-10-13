---
title: "Caching DNS server in a cluster"
permalink: en/virtualization-platform/documentation/architecture/network/dns-caching.html
---

Standard DNS operation in Kubernetes comes with a series of issues
that may cause degradation of key service performance indicators:

- By default, the Linux kernel does not cache DNS requests, so there is no local cache in Pods.
- All DNS requests from a container result in a network request to the cluster DNS.
  This is also true for requests to resources located on the same node.
- A Pod's DNS request is first resolved through cluster DNS zones, and only afterward sent to external DNS servers.
  For example, a request to `ya.com` will first be resolved through cluster zones like `cluster.local`,
  `svc.cluster.local`, and `<namespace>.svc.cluster.local`.
  Only after receiving negative responses (meaning, on the second attempt or later) it will be resolved correctly.

Any minor network delays can significantly degrade service quality due to the above-mentioned issues.

One possible solution is to install a DNS server on each node.
In Deckhouse Virtualization Platform, this is implemented using the [`node-local-dns`](/modules/node-local-dns/) module.

When using a caching DNS server,
external requests (that are not already cached) will still be attempted to resolve through the internal zone chain first.
Under high load (for example, with many repeated requests per second for the same records, which is common),
caching is enough to significantly improve DNS resolution performance.

## Caching DNS server operation principles

When the caching DNS server is deployed,
the [`node-local-dns`](/modules/node-local-dns/) module applies the following configuration steps on each cluster node:

- Configuring an interface with the IP address of the `kube-dns` service's clusterIP.
- Starting a caching CoreDNS that listens on that address.
- Adding an iptables rule: if the socket is open, traffic is redirected to it.
  Otherwise, a standard Kubernetes routing via ClusterIP is used:

  ```bash
  -A PREROUTING -d <kube-dns IP address> -m socket --nowildcard -j NOTRACK
  ```

### CoreDNS configuration aspects

Key characteristics of the CoreDNS configuration:

- All requests are cached.
- All DNS requests are forwarded to the cluster DNS ClusterIP.

## Grafana dashboard

The `Kubernetes / DNS (node local)` dashboard displays:

- General charts (providing an overall view of DNS performance),
- Per-node charts (helping investigate node-specific issues identified in the general charts),
- Upstream charts (helping evaluate performance of the cluster DNS and node DNS servers specified in `/etc/resolv.conf`).
