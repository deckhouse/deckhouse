---
title: Примеры конфигурации
permalink: ru/admin/configuration/logging/log-shipper/configuration-examples.html
lang: ru
---

## Базовые примеры

В этих примерах отражены основные варианты конфигурации модуля `log-shipper` для сбора логов в кластере,
включая работу с подами, фильтрацию по пространствам имён и лейблам и отправку данных в несколько хранилищ.

### Сбор логов со всех подов кластера и отправка в loki

Пример конфигурации `log-shipper` для сбора логов со всех подов кластера и их отправки в [`loki`](loki-overview.html)
с помощью кастомных ресурсов ClusterLoggingConfig и ClusterLogDestination (#TODO ссылка на CR).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
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

### Сбор логов с подов в указанном пространстве имён с указанным лейблом и отправка одновременно в loki и Elasticsearch

Пример конфигурации `log-shipper` для сбора логов с подов в пространстве имён `whispers` с лейблом `app=booking`
и их отправки одновременно в [`loki`](loki-overview.html) и Elasticsearch
с помощью кастомных ресурсов ClusterLoggingConfig и ClusterLogDestination (#TODO ссылка на CR).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: whispers-booking-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - whispers
    labelSelector:
      matchLabels:
        app: booking
  destinationRefs:
  - loki-storage
  - es-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: logs-%F
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

### Создание источника и сбор логов со всех подов в указанном пространстве имён с отправкой в loki

Пример конфигурации `log-shipper` для создания источника логов в пространстве имён `test-whispers`,
сбора логов со всех подов в этом пространстве имён
и их отправки в [`loki`](loki-overview.html) с помощью кастомных ресурсов PodLoggingConfig и ClusterLogDestination (#TODO ссылка на CR).

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

### Сбор логов только с подов в указанном пространстве имён и с указанным лейблом

Пример конфигурации `log-shipper` для сбора логов только с подов в пространстве имён `test-whispers`с лейблом `app=booking`
с помощью кастомных ресурсов PodLoggingConfig и ClusterLogDestination (#TODO ссылка на CR).

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

## Интеграция с внешними системами

В этом разделе представлены примеры настройки работы модуля `log-shipper` с Grafana, Splunk, Logstash и другими инструментами.

### Переход с Promtail на log-shipper

При миграции с Promtail на `log-shipper` необходимо скорректировать URL-адрес `loki`,
убрав из него путь `/loki/api/v1/push`.

Агент логирования Vector, который используется в `log-shipper`, автоматически добавит этот путь при отправке данных в Loki.

### Работа с Grafana Cloud

1. Создайте [ключ доступа к API Grafana Cloud](https://grafana.com/docs/grafana-cloud/reference/create-api-key/).
1. Закодируйте токен доступа к Grafana Cloud в формате Base64.

   ![API-ключ Grafana Cloud](../../images/log-shipper/grafana_cloud.png)

   ```bash
   echo -n "<YOUR-GRAFANACLOUD-TOKEN>" | base64 -w0
   ```

1. Создайте ресурс ClusterLogDestination (#TODO ссылка на CR), следуя примеру:

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

1. Cоздайте ресурс PodLoggingConfig или ClusterLoggingConfig (#TODO ссылка на CR), чтобы отправлять логи в Grafana Cloud.

### Добавление loki в Deckhouse Grafana

Чтобы работать с `loki` из Grafana, встроенной в Deckhouse, добавьте ресурс [GrafanaAdditionalDatasource](#TODO ссылка на CR).

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

### Поддержка Elasticsearch < 6.X

Для работы с версиями Elasticsearch ранее 6.0 включите поддержку индексов `docType` с помощью ресурса ClusterLogDestination:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    docType: "myDocType" # Укажите значение здесь. Оно не должно начинаться с '_'.
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

### Шаблон индекса для Elasticsearch

Существует возможность отправлять сообщения в определенные индексы на основе метаданных с помощью шаблонов индексов:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ namespace }}-%F"
```

В приведенном выше примере для каждого пространства имен Kubernetes будет создан свой индекс в Elasticsearch.

Эта функция также хорошо работает в комбинации с `extraLabels`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ service }}-{{ namespace }}-%F"
  extraLabels:
    service: "{{ service_name }}"
```

- Если сообщение имеет формат JSON, поле `service_name` этого документа JSON перемещается на уровень метаданных.
- Новое поле метаданных `service` используется в шаблоне индекса.

### Пример интеграции со Splunk

Модуль `log-shipper` поддерживает отправку событий в Splunk.

Настройка Splunk:

1. Определите endpoint. Он должен совпадать с именем вашего экземпляра Splunk с портом `8088`, но без указания пути,
   например, `https://prd-p-xxxxxx.splunkcloud.com:8088`.
1. Создайте токен для доступа. Для этого в Splunk откройте раздел **Setting** -> **Data inputs**,
   добавьте новый **HTTP Event Collector** и скопируйте сгенерированный токен.
1. Укажите индекс Splunk для хранения логов, например, `logs`.

Настройка `log-shipper`:

Добавьте ресурс ClusterLogDestination (#TODO ссылка на CR) для отправки логов в Splunk:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: splunk
spec:
  type: Splunk
  splunk:
    endpoint: https://prd-p-xxxxxx.splunkcloud.com:8088
    token: xxxx-xxxx-xxxx
    index: logs
    tls:
      verifyCertificate: false
      verifyHostname: false
```

{% alert level="info" %}
`destination` не поддерживает метки пода для индексирования.
Чтобы добавить нужные метки, используйте опцию `extraLabels`:

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

{% endalert %}

### Пример интеграции с Logstash

Чтобы настроить отправку логов в Logstash, выполните следующее:

1. Настройте входящий поток `tcp` с кодеком `json` на стороне Logstash. Пример конфигурации Logstash:

   ```hcl
   input {
     tcp {
       port => 12345
       codec => json
     }
   }
   output {
     stdout { codec => json }
   }
   ```

1. Добавьте ресурс ClusterLogDestination:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLogDestination
   metadata:
     name: logstash
   spec:
     type: Logstash
     logstash:
       endpoint: logstash.default:12345
   ```

### Пример интеграции с Graylog

Убедитесь, что в Graylog настроен входящий поток для приема сообщений по протоколу TCP на указанном порте.
Пример манифеста для интеграции с Graylog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-socket2-dest
spec:
  type: Socket
  socket:
    address: graylog.svc.cluster.local:9200
    mode: TCP
    encoding:
      codec: GELF
```

### Отправка сообщений в формате syslog

Используйте следующий пример конфигурации для отправки сообщений через сокет по протоколу TCP в формате syslog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: rsyslog
spec:
  type: Socket
  socket:
    mode: TCP
    address: 192.168.0.1:3000
    encoding: 
      codec: Syslog
  extraLabels:
    syslog.severity: "alert"
    # поле request_id должно присутствовать в сообщении
    syslog.message_id: "{{ request_id }}"
```

### Отправка сообщений в формате CEF

Модуль `log-shipper` поддерживает отправку логов в формате CEF через использование `codec: CEF`,
с переопределением `cef.name` и `cef.severity` по значениям из поля `message` лога приложения в формате JSON.

В примере ниже `app` и `log_level` — это ключи, содержащие значения для переопределения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: siem-kafka
spec:
  extraLabels:
    cef.name: '{{ app }}'
    cef.severity: '{{ log_level }}'
  type: Kafka
  kafka:
    bootstrapServers:
      - my-cluster-kafka-brokers.kafka:9092
    encoding:
      codec: CEF
    tls:
      verifyCertificate: false
      verifyHostname: true
    topic: logs
```

Также можно задать значения вручную:

```yaml
extraLabels:
  cef.name: 'TestName'
  cef.severity: '1'
```

## Фильтрация логов

...

## Буферизация логов

...

## Отладка и расширенные возможности

...

## Дополнительные настройки

...
