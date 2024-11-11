---
title: "The openvpn module: examples"
description: "Examples of the configuration of the openvpn Deckhouse Kubernetes Platform module for various scenarios, including bare metal clusters, AWS, Google Cloud, and public IP addresses on an external load balancer."
---

## An example for bare metal clusters

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
   inlet: ExternalIP
   externalIP: 5.4.54.4
```

## An example for AWS & Google Cloud

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    inlet: LoadBalancer
```

## An example for an external load balancer with a public IP address

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    externalHost: 5.4.54.4
    externalIP: 192.168.0.30 # The internal IP address to forward the external LB's traffic to.
    inlet: ExternalIP
    nodeSelector:
      kubernetes.io/hostname: node
```
