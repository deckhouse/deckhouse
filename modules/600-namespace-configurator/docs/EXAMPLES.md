---
title: "The namespace-configurator module: examples"
---

## Example

This example will add `extended-monitoring.flant.com/enabled=true` annotation and `foo=bar` label to every namespace starting with `prod-` and `infra-`, except `infra-test`.

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
