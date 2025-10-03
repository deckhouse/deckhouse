---
title: "Настройки логирования"
permalink: ru/user/logging/
lang: ru
search: logging configuration, log collection, log filtering, log shipping, centralized logging, настройка логирования, сбор логов, фильтрация логов, отправка логов, централизованное логирование
---

В Deckhouse Kubernetes Platform (DKP) предусмотрен сбор и доставка логов из узлов и подов кластера
во внутреннюю или внешние системы хранения.

DKP позволяет:

- собирать логи из всех или отдельных подов и пространств имён;
- фильтровать логи по лейблам, содержимому сообщений и другим признакам;
- направлять логи одновременно в несколько хранилищ (например, Loki и Elasticsearch);
- обогащать логи метаданными Kubernetes;
- использовать буферизацию логов для повышения производительности;
- хранить логи во внутреннем кратковременном хранилище на базе Grafana Loki.

Общий механизм сбора, доставки и фильтрации логов подробно описан [в разделе «Архитектура»](../../architecture/logging/delivery.html).

Пользователям DKP доступна настройка параметров сбора логов из приложения с помощью ресурса [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig), который описывает источник логов в рамках заданного пространства имён, включая правила сбора, фильтрации и парсинга.

## Настройка сбора логов из приложения

1. Уточните у администратора DKP, настроен ли сбор логов и хранилище в вашем кластере.
   Также попросите сообщить вам название хранилища, которое вы укажете в параметре [`clusterDestinationRefs`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-clusterdestinationrefs).
1. Создайте ресурс [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) в своём пространстве имён.

   В данном примере логи собираются со всех подов указанного пространства имён
   и отправляются в кратковременное хранилище [на базе Grafana Loki](../../admin/configuration/logging/storage.html):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: PodLoggingConfig
   metadata:
     name: app-logs
     namespace: my-namespace
   spec:
     clusterDestinationRefs:
       - loki-storage
   ```

1. (**Опционально**) Ограничьте сбор логов по лейблу.

   Если вам нужно собирать логи только с определённых подов,
   например, только от приложений с лейблом `app=backend`, добавьте [параметр `labelSelector`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-labelselector):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: PodLoggingConfig
   metadata:
     name: app-logs
     namespace: my-namespace
   spec:
     clusterDestinationRefs:
       - loki-storage
     labelSelector:
       matchLabels:
         app: backend
   ```

1. (**Опционально**) Настройте фильтрацию логов.

   Используя фильтры [`labelFilter`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-labelfilter) и [`logFilter`](/modules/log-shipper/cr.html#podloggingconfig-v1alpha1-spec-logfilter), вы можете установить фильтрацию по метаданным или полям сообщений.
   Например, в данном случае в хранилище отправятся лишь те логи, в которых нет полей со строкой `.*GET /status" 200$`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: PodLoggingConfig
   metadata:
     name: app-logs
     namespace: my-namespace
   spec:
     clusterDestinationRefs:
       - loki-storage
     labelSelector:
       matchLabels:
         app: backend
     logFilter:
     - field: message
       operator: NotRegex
       values:
       - .*GET /status" 200$
   ```

1. Примените созданный манифест с помощью следующей команды:

   ```shell
   d8 k apply -f pod-logging-config.yaml
   ```

## Примеры

### Создание source в пространстве имён и чтение логов всех подов в нём с направлением их в Loki

Следующий pipeline создает source в пространстве имён `test-whispers`, читает логи всех подов в нём и отправляет их в Loki:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

### Чтение подов в указанном пространстве имён с определенным лейблом

Пример настройки чтения подов с лейблом `app=booking` в пространстве имён `test-whispers`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  labelSelector:
    matchLabels:
      app: booking
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```
