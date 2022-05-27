---
title: "The openvpn module: usage"
---

## An example for bare metal clusters

```yaml
openvpnEnabled: "true"
openvpn: |
  inlet: ExternalIP
  externalIP: 5.4.54.4
```

## An example for AWS & Google Cloud

```yaml
openvpnEnabled: "true"
openvpn: |
  inlet: LoadBalancer
```

## An example for an external load balancer with a public IP address

```yaml
openvpnEnabled: "true"
openvpn: |
  externalHost: 5.4.54.4
  externalIP: 192.168.0.30 # The internal IP address to forward the external LB's traffic to.
  inlet: ExternalIP
  nodeSelector:
    kubernetes.io/hostname: node
```
