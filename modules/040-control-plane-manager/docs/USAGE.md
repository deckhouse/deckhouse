---
title: "Managing control plane: usage"
---

```yaml
controlPlaneManagerEnabled: "true"
controlPlaneManager: |
  bindToWildcard: true
  certSANs:
  - bakery.infra
  - devs.infra
  loadBalancer: {}
```
