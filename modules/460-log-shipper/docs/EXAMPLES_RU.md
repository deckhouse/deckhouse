---
title: "Модуль log-shipper: примеры"
description: Примеры использования модуля log-shipper Deckhouse. Примеры настройки модуля, фильтрации и сбора событий и логов в кластере Kubernetes.  
---

{% raw %}

## Чтение логов из всех подов кластера и направление их в Loki

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

## Чтение логов подов из указанного пространства имён с указанным label и перенаправление одновременно в Loki и Elasticsearch

Чтение логов подов из пространства имён `whispers` только с label `app=booking` и перенаправление одновременно в Loki и Elasticsearch:

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

## Создание source и чтение логов всех подов в пространстве имён с направлением их в Loki

Следующий pipeline создаёт source в пространстве имён `test-whispers`, читает логи всех подов в этом пространстве имён и записывает их в Loki:

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

## Чтение подов в указанном пространстве имён и с определённым label

Пример чтения подов в пространстве имён `test-whispers`, имеющих label `app=booking`:

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

Для начала вам потребуется закодировать в Base64 ваш токен доступа к Grafana Cloud.

![Grafana cloud API key](../../images/log-shipper/grafana_cloud.png)

```bash
echo -n "<YOUR-GRAFANA-CLOUD-TOKEN>" | base64 -w0
```

Затем нужно создать ресурс **ClusterLogDestination**:

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
      user: "<YOUR-GRAFANA-CLOUD-USER>"
    endpoint: <YOUR-GRAFANA-CLOUD-URL> # Например, https://logs-prod-us-central1.grafana.net или https://logs-prod-eu-west-0.grafana.net.
  type: Loki
```

Теперь можно создать PodLogginConfig или ClusterPodLoggingConfig и отправлять логи в **Grafana Cloud**.

## Добавление Loki в Deckhouse Grafana

Вы можете работать с Loki из встроенной в Deckhouse Grafana. Достаточно добавить [**GrafanaAdditionalDatasource**](../../modules/prometheus/cr.html#grafanaadditionaldatasource).

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

## Поддержка Elasticsearch < 6.X

Для Elasticsearch < 6.0 нужно включить поддержку индексов `doc_type`:

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

## Шаблон индекса для Elasticsearch

Существует возможность отправлять сообщения в определённые индексы на основе метаданных с помощью шаблонов индексов:

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

В приведённом выше примере для каждого пространства имён Kubernetes будет создан свой индекс в Elasticsearch.

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

1. Если сообщение имеет формат JSON, поле `service_name` этого документа JSON перемещается на уровень метаданных.
2. Новое поле метаданных `service` используется в шаблоне индекса.

## Пример интеграции со Splunk

Существует возможность отсылать события из Deckhouse в Splunk.

1. Endpoint должен быть таким же, как имя вашего экземпляра Splunk с портом `8088` и без указания пути, например `https://prd-p-xxxxxx.splunkcloud.com:8088`.
2. Чтобы добавить token для доступа, откройте пункт меню `Setting` -> `Data inputs`, добавьте новый `HTTP Event Collector` и скопируйте token.
3. Укажите индекс Splunk для хранения логов, например `logs`.

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

{% endraw %}
{% alert -%}
`destination` не поддерживает метки пода для индексирования. Рассмотрите возможность добавления нужных меток с помощью опции `extraLabels`.
{%- endalert %}
{% raw %}

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

## Простой пример Logstash

Чтобы отправлять логи в Logstash, на стороне Logstash должен быть настроен входящий поток `tcp` и его кодек должен быть `json`.

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

Пример манифеста ClusterLogDestination:

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

## Syslog

Следующий пример показывает, как отправлять сообщения через сокет по протоколу TCP в формате syslog:

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
    # В сообщении должно присутствовать поле `request_id`.
    syslog.message_id: "{{ request_id }}"
```

## Пример интеграции с Graylog

Убедитесь, что в Graylog настроен входящий поток для приема сообщений по протоколу TCP на указанном порту. Пример манифеста для интеграции с Graylog:

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

## Логи в CEF-формате

Существует способ формировать логи в формате CEF, используя `codec: CEF`, с переопределением `cef.name` и `cef.severity` по значениям из поля `message` (лога приложения) в формате JSON.

В примере ниже `app` и `log_level` – это ключи, содержащие значения для переопределения:

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

Также можно вручную задать свои значения:

```yaml
extraLabels:
  cef.name: 'TestName'
  cef.severity: '1'
```

## Сбор событий Kubernetes

События Kubernetes могут быть собраны модулем `log-shipper`, если `events-exporter` включён в настройках модуля [`extended-monitoring`](../extended-monitoring/).

Включите `events-exporter`, изменив параметры модуля `extended-monitoring`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: extended-monitoring
spec:
  version: 1
  settings:
    events:
      exporterEnabled: true
```

Выложите в кластер следующий `ClusterLoggingConfig`, чтобы собирать сообщения с пода `events-exporter`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubernetes-events
spec:
  type: KubernetesPods
  kubernetesPods:
    labelSelector:
      matchLabels:
        app: events-exporter
    namespaceSelector:
      matchNames:
      - d8-monitoring
  destinationRefs:
  - loki-storage
```

## Фильтрация логов

Пользователи могут применять следующие фильтры к логам:

* `labelFilter` — применяется к метаданным, например, к имени контейнера (`container`), пространству имён (`namespace`) или имени пода (`pod_name`);
* `logFilter` — применяется к полям самого сообщения, если оно в JSON-формате.

### Сборка логов только для контейнера `nginx`

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

### Сборка логов без строки, содержащей `GET /status" 200`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
  labelFilter:
  - field: message
    operator: NotRegex
    values:
    - .*GET /status" 200$
```

### Аудит событий kubelet

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
  - field: userAgent  
    operator: Regex
    values: ["kubelet.*"]
  destinationRefs:
  - loki-storage
```

### Системные логи Deckhouse

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: system-logs
spec:
  type: File
  file:
    include:
    - /var/log/syslog
  labelFilter:
  - field: message
    operator: Regex
    values:
    - .*d8-kubelet-forker.*
    - .*containerd.*
    - .*bashible.*
    - .*kernel.*
  destinationRefs:
  - loki-storage
```

{% endraw %}
{% alert -%}
Если вам нужны только логи одного пода или малой группы подов, постарайтесь использовать настройки `kubernetesPods`, чтобы ограничить количество читаемых файлов. Фильтры необходимы только для детализированной настройки.
{%- endalert %}
{% raw %}

## Настройка сборки логов с продуктовых пространств имён через опцию namespace label selector

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
        matchLabels:
          environment: production
  destinationRefs:
  - loki-storage
```

## Исключение подов и пространств имён с помощью label

Существует преднастроенный label для исключения определённых подов и пространств имён: `log-shipper.deckhouse.io/exclude=true`.
Он помогает остановить сбор логов с подов и пространств имён без изменения глобальной конфигурации.

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

## Исключение логов определённого контейнера внутри пода

Чтобы исключить сбор логов определённого контейнера внутри пода, добавьте для данного пода аннотацию следующего вида:

```yaml
vector.dev/exclude-containers: "container1,container2"
```

При данной аннотации Vector будет пропускать логи контейнеров с именем `container1` и `container2`.
Логи других контейнеров в поде продолжат собираться без каких-либо изменений.

Подробнее с этой функцией можно ознакомиться [в официальной документации Vector](https://vector.dev/docs/reference/configuration/sources/kubernetes_logs/#container-exclusion).

## Включение буферизации

Настройка буферизации логов необходима для улучшения надежности и производительности системы сбора логов. Буферизация может быть полезна в следующих случаях:

1. Временные перебои с подключением. Если есть временные перебои или нестабильность соединения с системой хранения логов (например, с Elasticsearch), буфер позволяет временно сохранять логи и отправить их, когда соединение восстановится.

1. Сглаживание пиков нагрузки. При внезапных всплесках объёма логов буфер позволяет сгладить пиковую нагрузку на систему хранения логов, предотвращая её перегрузку и потенциальную потерю данных.

1. Оптимизация производительности. Буферизация помогает оптимизировать производительность системы сбора логов за счёт накопления логов и отправки их группами, что снижает количество сетевых запросов и улучшает общую пропускную способность.

### Пример включения буферизации в оперативной памяти

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    memory:
      maxEvents: 4096
    type: Memory
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

### Пример включения буферизации на диске

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

### Пример определения поведения при переполнении буфера

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
    whenFull: DropNewest
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

Более подробное описание параметров доступно [в ресурсе ClusterLogDestination](cr.html#clusterlogdestination).

{% endraw %}
