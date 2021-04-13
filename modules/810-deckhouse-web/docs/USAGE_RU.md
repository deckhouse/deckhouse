---
title: "Модуль deckhouse-web: примеры конфигурации"
---

## Пример конфигурации модуля

```yaml
deckhouseWeb: |
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