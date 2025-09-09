---
title: "NLB"
permalink: en/user/network/ingress/nlb.html
---

NLB is provided by using Services of LoadBalancer type.

## Examples of Service configuration

### Shared IP address for multiple Services

To make Services use the same IP address, add the `network.deckhouse.io/load-balancer-shared-ip-key` annotation to them:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns-service-tcp
  namespace: default
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  ports:
    - name: dnstcp
      protocol: TCP
      port: 53
      targetPort: 53
  selector:
    app: dns
---
apiVersion: v1
kind: Service
metadata:
  name: dns-service-udp
  namespace: default
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  ports:
    - name: dnsudp
      protocol: UDP
      port: 53
      targetPort: 53
  selector:
    app: dns
```

### Forcing an IP address assignment

To enforce an IP address for a Service, add the `network.deckhouse.io/load-balancer-ips` annotation:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    network.deckhouse.io/load-balancer-ips: 192.168.217.217
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

### Assigning an IPAddressPool (BGP mode)

In BGP LoadBalancer mode, an IP address can be allocated from a specific address pool
using the `metallb.universe.tf/address-pool` annotation.
For L2 LoadBalancer mode, you need to use the [MetalLoadBalancerClass](../../../admin/configuration/network/ingress/nlb/metallb.html#example-of-using-metallb-in-l2-loadbalancer-mode) configuration.

Example:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.universe.tf/address-pool: production-public-ips
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```
