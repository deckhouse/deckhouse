---
title: Интеграция с службами Microsoft Azure
permalink: ru/admin/integrations/public/azure/azure-services.html
lang: ru
---

Deckhouse использует возможности облака Azure для полноценной интеграции с Kubernetes. При работе с кластером в Azure автоматически:

- Создаются сетевые маршруты для сети PodNetwork;
- Настраиваются внешние балансировщики нагрузки (LoadBalancer) для сервисов Kubernetes;
- Удаляются из кластера узлы, отсутствующие в облаке;
- Обновляются метаданные узлов в соответствии с текущей конфигурацией;
- Заказываются диски для узлов через CSI;
- Подключается необходимая CNI-сеть (используется simple bridge);
- Становятся доступны описания виртуальных машин в формате AzureInstanceClass для последующего использования в NodeGroup.

> Весь исходящий трафик из кластера идёт через балансировщики нагрузки. Если ни один из них не настроен для работы с UDP, весь исходящий UDP-трафик будет блокироваться, что может повлиять на работу NTP (`ntpdate`, `chrony` и др.). Решение: вручную добавить правило на любой UDP-порт в существующий LoadBalancer или создать Kubernetes-сервис типа LoadBalancer с UDP-портом.

## Поддержка Service Endpoints

Поддерживаются подключения к сервисам Azure через Service Endpoints:

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

Укажите необходимые сервисы в параметре `serviceEndpoints` объекта AzureClusterConfiguration.
