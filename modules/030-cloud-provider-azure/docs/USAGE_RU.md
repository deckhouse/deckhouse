---
title: "Cloud provider — Azure: примеры конфигурации"
---

## Пример CR `AzureInstanceClass`

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```
