---
title: Кратковременное хранение логов
permalink: ru/admin/configuration/logging/storage.html
lang: ru
---

Deckhouse предоставляет встроенное решение для кратковременного хранения логов на базе проекта [Grafana Loki](https://grafana.com/oss/loki/).

Хранилище разворачивается в кластере и интегрируется с системой сбора логов.
После настройки ресурсов ClusterLoggingConfig, PodLoggingConfig и ClusterLogDestination (#TODO ссылка на CR)
логи автоматически поступают из всех указанных источников.
Настроенное хранилище добавляется в Grafana в качестве источника данных для визуализации и анализа.

Параметры кратковременного хранилища задаются в настройках модуля `loki`(#TODO ссылка на параметры модуля).
Вы можете в том числе настроить размер диска и срок хранения, задать используемый StorageClass и ресурсы.

{% alert level="warning" %}
Кратковременное хранилище на базе Grafana Loki не поддерживает работу в режиме высокой доступности.
Для долговременного хранения важных логов используйте внешнее хранилище.
{% endalert %}

## Настройка кратковременного хранилища

Ниже приведён вариант конфигурации Deckhouse,
при котором логи из всех подов указанного пространства имён отправляются в хранилище на базе Loki.
При этом в настройках модуля `loki` указан StorageClass и определены размер диска для хранения логов и период хранения.

Для настройки выполните следующие шаги:

1. Включите модуль `loki`.
   Для этого используйте следующий манифест с настройками по умолчанию:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: loki
   spec:
     settings:
       storageClass: ceph-csi-rbd
       diskSizeGigabytes: 30
       retentionPeriodHours: 168
     enabled: true
     version: 1
   ```

   Модуль можно также активировать и настроить из веб-интерфейса Deckhouse.
   Для этого убедитесь, что у вас установлен модуль `console`(#TODO ссылка на доку про интерфейс),
   откройте веб-интерфейс Deckhouse и, выбрав `loki` в разделе **Модули**, включите его с помощью переключателя.

1. Создайте ресурс ClusterLoggingConfig(#TODO ссылка на CR), который задаёт правила сбора логов.
   Данный ресурс позволяет вам настроить сбор логов с подов в определенном пространстве имён и с определенным лейблом,
   гибко настраивать парсинг многострочных логов и задавать другие правила.

   В этом примере указывается, что нужно собирать логи из подов в пространстве имён `development`
   и отправлять их в кратковременное хранилище на базе Loki:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLoggingConfig
   metadata:
     name: development-logs
   spec:
     type: KubernetesPods
     kubernetesPods:
       namespaceSelector:
         matchNames:
           - development
     destinationRefs:
       - loki-storage
   ```

1. Создайте ресурс ClusterLogDestination(#TODO ссылка на CR), который описывает параметры отправки логов в хранилище.
   Данный ресурс позволяет вам указать одно или несколько хранилищ и описать параметры подключения, буферизации и дополнительные лейблы, которые будут применяться к логам перед отправкой.

   В этом примере в качестве принимающего хранилища указано кратковременное хранилище на базе Loki:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLogDestination
   metadata:
     name: loki-storage
   spec:
     type: Loki
     loki:
       endpoint: http://loki.loki:3100
   ```

## Интеграция с Grafana

Чтобы работать c Grafana, встроенной в Deckhouse, добавьте ресурс [GrafanaAdditionalDatasource](#TODO ссылка на CR).

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: loki
spec:
  access: Proxy
  basicAuth: false
  jsonData:
    maxLines: 5000
    timeInterval: 30s
  type: loki
  url: http://loki.loki:3100
```

### Grafana Cloud

Чтобы настроить работу Deckhouse с платформой Grafana Cloud, выполните следующие шаги:

1. создайте [ключ доступа к API Grafana Cloud](https://grafana.com/docs/grafana-cloud/reference/create-api-key/);
1. закодируйте токен доступа к Grafana Cloud в формате Base64:

   ![API-ключ Grafana Cloud](../../images/log-shipper/grafana_cloud.png)

   ```bash
   echo -n "<YOUR-GRAFANACLOUD-TOKEN>" | base64 -w0
   ```

1. создайте ресурс ClusterLogDestination (#TODO ссылка на CR), следуя примеру:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLogDestination
   metadata:
     name: loki-storage
   spec:
     loki:
       auth:
         password: PFlPVVItR1JBRkFOQUNMT1VELVRPS0VOPg==
         strategy: Basic
         user: "<YOUR-GRAFANACLOUD-USERNAME>"
       endpoint: <YOUR-GRAFANACLOUD-URL> # Например https://logs-prod-us-central1.grafana.net или https://logs-prod-eu-west-0.grafana.net
     type: Loki
   ```

### Переход с Grafana Promtail

Для миграции с Promtail отредактируйте URL-адрес Loki, убрав из него путь `/loki/api/v1/push`.

Агент логирования Vector, который используется в Deckhouse, автоматически добавит этот путь при отправке данных в Loki.
