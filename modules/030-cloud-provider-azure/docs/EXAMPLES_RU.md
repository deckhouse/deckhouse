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

### Network Security Group по умолчанию

Модуль создаёт Network Security Group (NSG) с именем префикса кластера и привязывает её к подсети узлов. В NSG по умолчанию добавляются правила:

* `AllowIcmp` — входящий ICMP из любого источника;
* `AllowSsh` — входящий TCP/22 из CIDR в [`sshAllowList`](cluster_configuration.html#azureclusterconfiguration-sshallowlist) (если список не задан — из любого источника).

Дополнительные порты (например, HTTP/HTTPS) модуль в эту NSG не добавляет. Их нужно настроить вручную в Azure (правила NSG или отдельная NSG). Подключить заранее созданную собственную NSG к узлам через параметры модуля нельзя — доступен только [`sshAllowList`](cluster_configuration.html#azureclusterconfiguration-sshallowlist) для ограничения SSH.
