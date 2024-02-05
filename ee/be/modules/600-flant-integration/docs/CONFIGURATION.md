---
title: "The flant-integration module: configuration"
---

<!-- SCHEMA -->

## Example of configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: flant-integration
spec:
  version: 1
  enabled: true
  settings:
    licenseKey: s6f8766314a9426faa2b3
    kubeall:
      host: myproject.kube-master-0
      kubeconfig: /etc/kubernetes/admin.conf
```
