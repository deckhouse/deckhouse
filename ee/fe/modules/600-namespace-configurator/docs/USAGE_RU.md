---
title: "Модуль namespace-configurator: примеры конфигурации"
---

## Пример:

Этот пример добавит аннотацию `extended-monitoring.flant.com/enabled=true` и label `foo=bar` к каждому Namespace, начинающемуся с `prod-` или `infra-`, за исключением `infra-test`.

```yaml
namespaceConfiguratorEnabled: "true"
namespaceConfigurator: |
  configurations:
  - annotations:
      extended-monitoring.flant.com/enabled: "true"
    labels:
      foo: bar
    includeNames:
    - "prod-.*"
    - "infra-.*"
    excludeNames:
    - "infra-test"
```
