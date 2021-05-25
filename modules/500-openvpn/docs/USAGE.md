---
title: "The openvpn module: usage"
---

## An example for bare metal clusters

```
openvpnEnabled: "true"
openvpn: |
  inlet: ExternalIP
  externalIP: 5.4.54.4
```

## An example for AWS & Google Cloud

```
openvpnEnabled: "true"
openvpn: |
  inlet: LoadBalancer
```

## An example for an external load balancer with a public IP address

```
openvpnEnabled: "true"
openvpn: |
  externalHost: 5.4.54.4
  externalIP: 192.168.0.30 # the internal IP address to forward the external LB's traffic to
  inlet: ExternalIP
  nodeSelector:
    kubernetes.io/hostname: node
```
