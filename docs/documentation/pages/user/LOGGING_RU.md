---
title: Сбор логов из приложения
permalink: ru/user/monitoring/logging.html
lang: ru
---

В Deckhouse Kubernetes Platform (DKP) предусмотрен сбор и доставка логов из узлов и подов кластера во внутреннюю или внешние системы хранения.

DKP позволяет:

- собирать логи из всех или отдельных подов и пространств имён;
- фильтровать логи по лейблам, содержимому сообщений и другим признакам;
- направлять логи одновременно в несколько хранилищ (например, Loki и Elasticsearch);
- обогащать логи метаданными Kubernetes;
- использовать буферизацию логов для повышения производительности;
- хранить логи во внутреннем кратковременном хранилище на базе Grafana Loki.

Общий механизм сбора, доставки и фильтрации логов подробно описан [в разделе «Архитектура»](#TODO ссылка на Архитектура -> Логирование).

Для настройки сбора и доставки логов используются кастомные ресурсы ClusterLoggingConfig,
PodLoggingConfig и ClusterLogDestination.
Администраторам DKP доступны для настройки все параметры отправки и приема логов(#TODO ссылка на Администрирование -> Логирование -> Сбор и доставка логов).
Пользователи кластера могут указать, какие логи следует собирать в пределах пространства имён с помощью правил фильтрации и парсинга логов, используя ресурс PodLoggingConfig.

Все доступные параметры ресурса PodLoggingConfig описаны [в разделе «Справка»](#TODO ссылка на Reference -> CR).

## Настройка сбора логов из приложения

1. Уточните у администратора DKP, настроен ли сбор логов и хранилище в вашем кластере.
   Также попросите сообщить вам название хранилища, которое вы укажете в параметре `clusterDestinationRefs`(#TODO ссылка на CR).
1. Создайте ресурс PodLoggingConfig в своём пространстве имён.

   В данном примере логи собираются со всех подов указанного пространства имён
   и отправляются в кратковременное хранилище на базе Grafana Loki(#TODO ссылка на Администрирование -> Логирование -> Кратковременное хранение логов):

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
   например, только от приложений с лейблом `app=backend`, добавьте параметр `labelSelector` (#TODO ссылка на CR):

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

   Используя фильтры `labelFilter` и `logFilter` (#TODO ссылка на CR), вы можете установить фильтрацию по метаданным или полям сообщений.
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
   sudo -i d8 k apply -f pod-logging-config.yaml
   ```
