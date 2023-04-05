---
title: "Модуль namespace-configurator: примеры"
---

## Пример

Этот пример добавит аннотацию `extended-monitoring.flant.com/enabled=true` и label `foo=bar` к каждому Namespace, начинающемуся с `prod-` или `infra-`, за исключением `infra-test`.

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
        extended-monitoring.flant.com/enabled: "true"
      labels:
        foo: bar
      includeNames:
      - "^prod"
      - "^infra"
      excludeNames:
      - "infra-test"
```
