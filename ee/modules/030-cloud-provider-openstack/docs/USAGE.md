---
title: "Сloud provider — OpenStack: usage"
---

## An example of the `OpenStackInstanceClass` CR

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
```
