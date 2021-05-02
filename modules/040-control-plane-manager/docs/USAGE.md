---
title: "Managing control plane: configuration examples"
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
