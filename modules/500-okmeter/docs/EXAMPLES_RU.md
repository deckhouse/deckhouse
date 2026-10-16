---
title: "Модуль okmeter: примеры"
---

## Пример

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: okmeter
spec:
  version: 1
  enabled: true
  settings: 
    apiKey: dh-test-secret-0123456789abcdef
```
