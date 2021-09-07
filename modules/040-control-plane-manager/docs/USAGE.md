---
title: "Managing control plane: usage"
---

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
