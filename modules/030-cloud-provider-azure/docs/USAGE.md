---
title: "Сloud provider — Azure: usage"
---

## An example of the `AzureInstanceClass` CR

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```
