---
title: "Управление control plane: примеры конфигурации"
---

Ниже представен простой пример конфигурации control plane:

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
