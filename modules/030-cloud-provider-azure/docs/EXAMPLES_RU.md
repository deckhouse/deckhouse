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

## Настройка политик безопасности на узлах

Правила Network Security Group по умолчанию описаны в разделе [схем размещения](layouts.html#network-security-group). Подключить заранее созданную собственную NSG к узлам через параметры модуля нельзя — доступен только [`sshAllowList`](cluster_configuration.html#azureclusterconfiguration-sshallowlist) для ограничения SSH.
