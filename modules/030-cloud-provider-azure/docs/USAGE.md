---
title: "Cloud provider — Azure: usage"
---

## An example of the `AzureInstanceClass` custom resource

Below is a simple example of custom resource `AzureInstanceClass` configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```
