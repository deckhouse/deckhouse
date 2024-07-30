---
title: "Модуль dashboard: примеры"
---

## Пример конфигурации модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/example: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: example
    externalAuthentication:
      authURL: "https://<applicationDomain>/auth"
      authSignInURL: "https://<applicationDomain>/sign-in"
      authResponseHeaders: "Authorization"
```
