---
title: "Managing control plane: examples"
---

## Control plane module config

Below is a simple control plane configuration example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      bindToWildcard: true
      certSANs:
      - bakery.infra
      - devs.infra
      loadBalancer: {}
```

## Connect extender for kube-scheduler

```yaml
apiVersion: deckhouse.io/v1alpha1
type: KubeSchedulerWebhookConfiguration
metadata:
name: sds-replicated-volume
Webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
  service:
  name: scheduler
  namespace: d8-sds-replicated-volume
  port: 8080
  path: /scheduler
  caBundle: ABCD=
  timeoutSeconds: 5
```
