---
title: "Cloud provider — Azure: примеры"
---

## Пример custom resource `AzureInstanceClass`

Ниже представлен простой пример custom resource `AzureInstanceClass`:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```
