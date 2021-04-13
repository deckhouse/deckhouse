---
title: "Управление control plane: примеры конфигурации"
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
