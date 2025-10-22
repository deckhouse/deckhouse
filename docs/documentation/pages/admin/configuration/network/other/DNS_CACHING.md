---
title: "Caching DNS requests on cluster nodes"
permalink: en/admin/configuration/network/other/dns-caching.html
---

In Deckhouse Kubernetes Platform, you can deploy a local caching DNS server on each cluster node.
It exports metrics to Prometheus for visualization in a [Grafana dashboard](/modules/node-local-dns/#grafana-dashboard).

This feature is implemented by the [`node-local-dns`](/modules/node-local-dns/) module.
The module consists of the original CoreDNS deployed as a DaemonSet on all cluster nodes,
along with a network configuration algorithm and iptables rules.

Detailed information about the problems that caching DNS-server allows to solve and the principle of its work is available in [Caching DNS server in a cluster](../../../../architecture/network/dns-caching.html).

## Example custom DNS configuration in a Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dns-example
spec:
  dnsPolicy: "None"
  dnsConfig:
    nameservers:
      - 169.254.20.10
  containers:
    - name: test
      image: nginx
```

For details on DNS configuration, refer to the [Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config).
