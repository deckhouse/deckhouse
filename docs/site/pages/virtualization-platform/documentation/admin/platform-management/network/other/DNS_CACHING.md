---
title: "Caching DNS requests on cluster nodes"
permalink: en/virtualization-platform/documentation/admin/platform-management/network/other/dns-caching.html
---

In Deckhouse Virtualization Platform, you can deploy a local caching DNS server on each cluster node.
It exports metrics to Prometheus for visualization in a [Grafana dashboard](/products/kubernetes-platform/documentation/v1/modules/node-local-dns/#grafana-dashboard).

This feature is implemented by the [`node-local-dns`](/products/kubernetes-platform/documentation/v1/modules/node-local-dns/) module.
The module consists of the original CoreDNS deployed as a DaemonSet on all cluster nodes,
along with a network configuration algorithm and iptables rules.

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
