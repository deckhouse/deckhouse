---
title: "The namespace-configurator module: examples"
---

## Example

Add the label `extended-monitoring.deckhouse.io/enabled=true` and the annotation `foo=bar` to every namespace starting with `prod-` and `infra-`, except `infra-test`.

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
