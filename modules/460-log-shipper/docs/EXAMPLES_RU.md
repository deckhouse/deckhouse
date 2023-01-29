---
title: "Модуль log-shipper: примеры"
---

{% raw %}
## Чтение логов из всех Pod'ов кластера и направление их в Loki

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

## Чтение логов Pod'ов из указанного namespace с указанным label и перенаправление одновременно в Loki и Elasticsearch

Чтение логов Pod'ов из namespace `whispers` только с label `app=booking` и перенаправление одновременно в Loki и Elasticsearch:

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

## Создание source в namespace и чтение логов всех Pod'ов в этом NS, с направлением их в Loki

Следующий pipeline создает source в namespace: `test-whispers` и читает логи всех Pod'ов в этом NS, пишет их в Loki:

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

## Чтение только Pod'ов в указанном namespace и имеющих определенный label

Пример чтения только Pod'ов, имеющих label `app=booking` в namespace `test-whispers`:

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

## Переход с Promtail на Log-Shipper

В ранее используемом URL Loki требуется убрать путь `/loki/api/v1/push`.

**Vector** сам добавит этот путь при работе с Loki.

## Работа с Grafana Cloud

Данная документация подразумевает, что у вас уже [создан ключ API](https://grafana.com/docs/grafana-cloud/reference/create-api-key/).

Для начала вам потребуется закодировать в base64 ваш токен доступа к GrafanaCloud.

![Grafana cloud API key](../../images/460-log-shipper/grafana_cloud.png)

```bash
echo -n "<YOUR-GRAFANACLOUD-TOKEN>" | base64 -w0
```

Затем создадим **ClusterLogDestination**

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
      user: "<YOUR-GRAFANACLOUD-USER>"
    endpoint: <YOUR-GRAFANACLOUD-URL> # Например https://logs-prod-us-central1.grafana.net или https://logs-prod-eu-west-0.grafana.net
  type: Loki
```

Теперь можно создать PodLogginConfig или ClusterPodLoggingConfig и отправлять логи в **Grafana Cloud**.

## Добавление Loki в Deckhouse Grafana

Вы можете работать с Loki из встроенной в Deckhouse Grafana. Достаточно добавить [**GrafanaAdditionalDatasource**](../../modules/300-prometheus/cr.html#grafanaadditionaldatasource)

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

## Поддержка elasticsearch < 6.X

Для Elasticsearch < 6.0 нужно включить поддержку doc_type индексов.
Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    docType: "myDocType" # Укажите значение здесь. Оно не должно начинаться с '_'
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

## Пример интеграции со Splunk

Существует возможность отсылать события из Deckhouse в Splunk.

1. Endpoint должен быть таким же как имя вашего экземпляра Splunk с портом `8088` и без указания пути, например, `https://prd-p-xxxxxx.splunkcloud.com:8088`.
2. Чтобы добавить token для доступа, откройте пункт меню `Setting` -> `Data inputs`, добавьте новый `HTTP Event Collector` и скопируйте token.
3. Укажите индекс Splunk для хранения логов, например, `logs`.

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

> NOTE: Splunk destination не поддерживает метки Pod'а для индексирования. Рассмотрите возможность добавления нужных меток при помощи опции `extraLabels`.

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

## Простой пример Logstash

Чтобы отправлять логи в Logstash, на стороне Logstash должен быть настроен входящий поток `tcp`, и его кодек должен быть — `json`.

Пример минимальной конфигурации Logstash:

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

Пример манифеста `ClusterLogDestination`:

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

## Фильтрация логов

Только логи контейнера Nginx:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: nginx-logs
spec:
  type: KubernetesPods
  labelFilter:
  - field: container
    operator: In
    values: [nginx]
  destinationRefs:
  - loki-storage
```

Не debug и не JSON-логи:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: non-debug-logs
spec:
  logFilter:
  - operator: NotRegex
    values: ["DEBUG.*"]
  destinationRefs:
  - loki-storage
```

Только ошибки микросервисов бекэнда:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: backend-logs
spec:
  type: KubernetesPods
  labelFilter:
    - field: pod_labels.app
      operator: In
      values: [web-server, queue-worker]
  logFilter:
    - field: error
      operator: Exists
  destinationRefs:
    - loki-storage
```

> NOTE: Если вам нужны только логи одного или малой группы pod'ов, постарайтесь использовать настройки kubernetesPods, чтобы сузить количество читаемых файлов. Фильтры необходимы только для высокогранулярной настройки.

## Настройка сборки логов с продуктовых namespace'ов используя опцию namespace label selector

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: production-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchNames:
          environment: production
  destinationRefs:
  - loki-storage
```

## Исключить Pod'ы и namespace'ы используя label

Существует преднастроенный label для исключения определенных Pod'ов и namespace'ов: `log-shipper.deckhouse.io/exclude=true`.
Он помогает остановить сбор логов с Pod'ов и namespace'ов без изменения глобальной конфигурации.

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
  labels:
    log-shipper.deckhouse.io/exclude: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  ...
  template:
    metadata:
      labels:
        log-shipper.deckhouse.io/exclude: "true"
```
{% endraw %}
