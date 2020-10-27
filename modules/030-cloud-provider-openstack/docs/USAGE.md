---
title: "Сloud provider — OpenStack: примеры конфигурации"
---

## Пример CR `OpenStackInstanceClass`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
```
