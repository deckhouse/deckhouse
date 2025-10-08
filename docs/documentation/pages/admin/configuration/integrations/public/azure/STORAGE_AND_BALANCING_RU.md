---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/azure/storage.html
lang: ru
---

## Хранилище

При работе с Azure Deckhouse Kubernetes Platform (DKP) автоматически создаёт следующие StorageClass:

| Имя                    | Тип диска        |
| ---------------------- | ---------------- |
| `managed-standard`     | Standard\_LRS    |
| `managed-standard-ssd` | StandardSSD\_LRS |
| `managed-premium`      | Premium\_LRS     |

Дополнительно можно:

- Отключить ненужные классы хранилища через [параметр `exclude`](/modules/cloud-provider-azure/configuration.html#parameters-storageclass-exclude).
- Создать свои StorageClass с нужными параметрами пропускной способности и IOPS.

Пример настройки в ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-azure
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      provision:
      - name: managed-ultra-ssd
        type: UltraSSD_LRS
        diskIOPSReadWrite: 600
        diskMBpsReadWrite: 150
      exclude:
      - managed-standard.*
      - managed-premium
```

## Балансировка нагрузки

DKP автоматически создает ресурсы LoadBalancer в Azure при использовании Kubernetes-сервисов типа LoadBalancer.

Дополнительные особенности:

- Поддерживается использование NAT Gateway — можно явно задать количество публичных IP-адресов для SNAT через [параметр `natGatewayPublicIpCount`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-standard-natgatewaypublicipcount).
- Возможна настройка пиринга VNet, в том числе с bastion-хостом или другими VNet в облаке.
- Поддерживаются Service Endpoints — безопасное и прямое подключение к сервисам Azure без использования публичных IP. Подробнее – в разделе [Интеграция с сервисами Azure](services.html).
