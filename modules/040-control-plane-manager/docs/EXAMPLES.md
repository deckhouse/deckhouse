---
title: "Managing control plane: examples"
---

Below is a simple control plane configuration example:

```yaml
controlPlaneManagerEnabled: "true"
controlPlaneManager: |
  apiserver:
    bindToWildcard: true
    certSANs:
    - bakery.infra
    - devs.infra
    loadBalancer: {}
```
