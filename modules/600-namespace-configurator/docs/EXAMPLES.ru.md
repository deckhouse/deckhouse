---
title: "Модуль namespace-configurator: примеры"
---

## Пример

Добавьте лейбл `extended-monitoring.deckhouse.io/enabled=true` и аннотацию `foo=bar` к каждому namespace, начинающемуся с `prod-` или `infra-`, за исключением `infra-test`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: namespace-configurator
spec:
  version: 1
  enabled: true
  settings:
    configurations:
    - annotations:
        foo: bar
      labels:
        extended-monitoring.deckhouse.io/enabled: "true"
      includeNames:
      - "^prod"
      - "^infra"
      excludeNames:
      - "infra-test"
```
