---
title: "Модуль descheduler: примеры"
---

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: descheduler
spec:
  version: 1
  enabled: true
  settings:
    removePodsViolatingNodeAffinity: false
    removeDuplicates: true
    lowNodeUtilization: true
    nodeSelector:
      node-role/example: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: example
```
