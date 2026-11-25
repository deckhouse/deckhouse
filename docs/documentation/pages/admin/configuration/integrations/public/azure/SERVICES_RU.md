---
title: Интеграция со службами Microsoft Azure
permalink: ru/admin/integrations/public/azure/services.html
lang: ru
---

## Поддержка Service Endpoints

Поддерживаются подключения к сервисам Azure через Service Endpoints, которые:

- Позволяют осуществлять подключение к сервисам Azure без использования публичных IP-адресов.
- Работают по оптимизированному маршруту через магистральную сеть Azure.
- Упрощают контроль доступа и повышают безопасность.

Список поддерживаемых Service Endpoints:

```console
Microsoft.AzureActiveDirectory
Microsoft.AzureCosmosDB
Microsoft.ContainerRegistry
Microsoft.CognitiveServices
Microsoft.EventHub
Microsoft.KeyVault
Microsoft.ServiceBus
Microsoft.Sql
Microsoft.Storage
Microsoft.Storage.Global
Microsoft.Web
```

Укажите необходимые сервисы в [параметре `serviceEndpoints`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-serviceendpoints) объекта AzureClusterConfiguration.
