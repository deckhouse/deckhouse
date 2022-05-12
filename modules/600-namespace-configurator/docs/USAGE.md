---
title: "The namespace-configurator module: usage"
---

## Example

This example will add `extended-monitoring.flant.com/enabled=true` annotation and `foo=bar` label to every namespace starting with `prod-` and `infra-`, except `infra-test`.

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
