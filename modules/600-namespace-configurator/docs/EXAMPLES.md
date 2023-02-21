---
title: "The namespace-configurator module: examples"
---

## Example

This example will add `extended-monitoring.deckhouse.io/enabled=true` label and `foo=bar` label to every namespace starting with `prod-` and `infra-`, except `infra-test`.

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
        extended-monitoring.flant.com/enabled: "true"
      includeNames:
      - "prod-.*"
      - "infra-.*"
      excludeNames:
      - "infra-test"
```
