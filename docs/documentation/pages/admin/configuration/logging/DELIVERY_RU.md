---
title: Сбор и доставка логов
permalink: ru/admin/configuration/logging/delivery.html
description: "Настройка сбора и доставки логов в Deckhouse Kubernetes Platform. Централизованное логирование из подов и узлов во внутренние или внешние системы хранения с фильтрацией и маршрутизацией."
lang: ru
---

В Deckhouse Kubernetes Platform (DKP) предусмотрен сбор и доставка логов из узлов и подов кластера во внутреннюю или внешние системы хранения.

DKP позволяет:

- собирать логи из всех или отдельных подов и пространств имён;
- фильтровать логи по лейблам, содержимому сообщений и другим признакам;
- направлять логи одновременно в несколько хранилищ (например, Loki и Elasticsearch);
- обогащать логи метаданными Kubernetes;
- использовать буферизацию логов для повышения производительности.

Общий механизм сбора, доставки и фильтрации логов подробно описан [в разделе «Архитектура»](../../../architecture/logging/delivery.html).

Администраторам DKP доступна настройка сбора и отправки логов с помощью трёх кастомных ресурсов:

- [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) — описывает источник логов на уровне кластера,
  включая правила сбора, фильтрации и парсинга;
- [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) — описывает источник логов
  в рамках заданного пространства имён, включая правила сбора, фильтрации и парсинга;
- [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) — задаёт параметры хранилища логов.

На основе этих ресурсов формируется *pipeline*, который используется в DKP для чтения логов
и дальнейшей работы с ними c помощью [модуля `log-shipper`](/modules/log-shipper/).
Полный перечень настроек модуля `log-shipper` доступен [в отдельном разделе документации](/modules/log-shipper/configuration.html).

## Настройка сбора и доставки логов

Ниже приведён вариант базовой конфигурации DKP,
при котором логи со всех подов кластера отправляются в хранилище на базе Elasticsearch.

Для настройки выполните следующие шаги:

1. Включите [модуль `log-shipper`](/modules/log-shipper/) с помощью следующей команды:

   ```shell
   d8 platform module enable log-shipper
   ```

1. Создайте ресурс [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), который задаёт правила сбора логов.
   Данный ресурс позволяет вам настроить сбор логов с подов в определенном пространстве имён и с определенным лейблом,
   гибко настраивать парсинг многострочных логов и задавать другие правила.

   В этом примере указывается, что нужно собирать логи со всех подов и отправлять их в Elasticsearch:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLoggingConfig
   metadata:
     name: all-logs
   spec:
     type: KubernetesPods
     destinationRefs:
     - es-storage
   ```

1. Создайте ресурс [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination),
   который описывает параметры отправки логов в хранилище.
   Данный ресурс позволяет вам указать одно или несколько хранилищ и описать параметры подключения, буферизации и дополнительные лейблы, которые будут применяться к логам перед отправкой.

   В этом примере в качестве принимающего хранилища указан Elasticsearch:

   ```yaml
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

## Интеграция с внешними системами

Вы можете настроить DKP на работу с внешними системами хранения и анализа логов,
такими как Elasticsearch, Splunk, Logstash и другими,
используя [параметр `type`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-type) ресурса ClusterLogDestination.

### Elasticsearch

Чтобы отправлять логи в Elasticsearch, создайте ресурс [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination), следуя этому примеру:

```yaml
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

#### Использование шаблона индексов

Чтобы отправлять сообщения в определенные индексы на основе метаданных с помощью шаблонов индексов,
используйте следующую конфигурацию:

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

В приведенном примере для каждого пространства имён Kubernetes будет создан свой индекс в Elasticsearch.

Эта функция удобна в комбинации [с параметром `extraLabels`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-extralabels):

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

#### Поддержка Elasticsearch < 6.X

Для работы с версиями Elasticsearch ранее 6.0 включите поддержку [индексов `docType`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-elasticsearch-doctype) с помощью ресурса ClusterLogDestination:

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

### Splunk

Чтобы настроить отправку событий в Splunk, выполните следующие шаги:

1. Настройте Splunk:
   - Определите endpoint. Он должен совпадать с именем вашего экземпляра Splunk с портом `8088`, но без указания пути,
   например, `https://prd-p-xxxxxx.splunkcloud.com:8088`.
   - Создайте токен для доступа. Для этого в Splunk откройте раздел **Setting** -> **Data inputs**,
   добавьте новый **HTTP Event Collector** и скопируйте сгенерированный токен.
   - Укажите индекс Splunk для хранения логов, например, `logs`.

1. Настройте DKP, добавив ресурс [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination) для отправки логов в Splunk:

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
Чтобы добавить нужные метки, используйте [опцию `extraLabels`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-extralabels):

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

{% endalert %}

### Logstash

Чтобы настроить отправку логов в Logstash, выполните следующее:

1. Настройте входящий поток `tcp` с кодеком `json` на стороне Logstash.

   Пример конфигурации Logstash:

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

1. Добавьте ресурс [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination):

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

### Graylog

Чтобы настроить отправку логов в Graylog, выполните следующее:

1. Убедитесь, что в Graylog настроен входящий поток для приема сообщений по протоколу TCP на указанном порте.
1. Создайте ресурс [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination), следуя примеру:

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

## Формат сообщений

Вы можете выбрать формат отправляемых сообщений, используя [параметр `.encoding.codec`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-socket-encoding-codec) ресурса ClusterLogDestination:

- CEF
- GELF
- JSON
- Syslog
- Text

Ниже примеры конфигурации для некоторых из них.

### Syslog

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
    # Поле request_id должно присутствовать в сообщении.
    syslog.message_id: "{{ request_id }}"
```

### CEF

DKP может отправлять логи в формате CEF через использование `codec: CEF`,
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

## Преобразование логов

Вы можете настроить один или несколько видов трансформаций, которые будут применяться к логам перед отправкой в хранилище.

### Преобразование записи в структурированный объект

Трансформация [`ParseMessage`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-parsemessage) позволяет преобразовать строку в поле `message` в структурированный JSON-объект
на основе одного или нескольких заданных форматов (String, Klog, SysLog и другие).

{% alert level="warning" %}
При использовании нескольких трансформаций `ParseMessage`
преобразование строки (`sourceFormat: String`) должно выполняться в последнюю очередь.
{%- endalert %}

Пример настройки преобразования записей смешанных форматов:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLogDestination
metadata:
  name: parse-json
spec:
  ...
  transformations:
  - action: ParseMessage
    parseMessage:
      sourceFormat: JSON
  - action: ParseMessage
    parseMessage:
      sourceFormat: Klog
  - action: ParseMessage
    parseMessage:
      sourceFormat: String
      string:
        targetField: "text"
```

Пример изначальной записи в логе:

```text
/docker-entrypoint.sh: Configuration complete; ready for start up
{"level" : { "severity": "info" },"msg" : "fetching.module.release"}
I0505 17:59:40.692994   28133 klog.go:70] hello from klog
```

Результат преобразования:

```json
{... "message": {
  "text": "/docker-entrypoint.sh: Configuration complete; ready for start up"
  }
}
{... "message": {
  "level" : "{ "severity": "info" }",
  "msg" : "fetching.module.release"
  }
}
{... "message": {
  "file":"klog.go",
  "id":28133,
  "level":"info",
  "line":70,
  "message":"hello from klog",
  "timestamp":"2025-05-05T17:59:40.692994Z"
  }
}
```

### Замена лейблов

Трансформация [`ReplaceKeys`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-replacekeys) позволяет рекурсивно заменить все совпадения шаблона `source` на значение `target` в указанных ключах лейблов.

{% alert level="warning" %}
Перед применением трансформации `ReplaceKeys` к полю `message` или его вложенным полям
преобразуйте запись лога в структурированный объект с помощью трансформации [`ParseMessage`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-parsemessage).
{%- endalert %}

Пример настройки замены точек на нижние подчеркивания в лейблах:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLogDestination
metadata:
  name: replace-dot
spec:
  ...
  transformations:
    - action: ReplaceKeys
      replaceKeys:
        source: "."
        target: "_"
        labels:
          - .pod_labels
```

Пример изначальной записи в логе:

```json
{"msg" : "fetching.module.release"} # Лейбл пода pod.app=test
```

Результат преобразования:

```json
{... "message": {
  "msg" : "fetching.module.release"
  },
  "pod_labels": {
    "pod_app": "test"
  }
}
```

### Удаление лейблов

Трансформация [`DropLabels`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-droplabels) позволяет удалить указанные лейблы из структурированного JSON-сообщения.

{% alert level="warning" %}
Перед применением трансформации `DropLabels` к полю `message` или его вложенным полям
преобразуйте запись лога в структурированный объект с помощью трансформации [`ParseMessage`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-transformations-parsemessage).
{%- endalert %}

Пример конфигурации с удалением лейбла и предварительной трансформацией `ParseMessage`:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLogDestination
metadata:
  name: drop-label
spec:
  ...
  transformations:
    - action: ParseMessage
      parseMessage:
        sourceFormat: JSON
    - action: DropLabels
      dropLabels:
        labels:
          - .message.example
```

Пример изначальной записи в логе:

```json
{"msg" : "fetching.module.release", "example": "test"}
```

Результат преобразования:

```json
{... "message": {
  "msg" : "fetching.module.release"
  }
}
```

## Фильтрация логов

В DKP предусмотрены фильтры, позволяющие исключить лишние сообщения для оптимизации процесса сбора логов:

- [`labelFilter`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-labelfilter) — применяется к метаданным,
  например, к имени контейнера (`container`), пространству имён (`namespace`) или имени пода (`pod_name`);
- [`logFilter`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-logfilter) — применяется к полям сообщения,
  если оно в JSON-формате.

### Сборка логов из определенного контейнера

Чтобы настроить фильтрацию с помощью `labelFilter`,
создайте ресурс [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig),
используя конфигурацию ниже в качестве примера.

В этом случае фильтр отбирает логи из контейнеров с именем `nginx`,
а затем отправляет их во внутреннее хранилище на базе Loki.

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

### Сборка логов без заданной строки

Пример конфигурации для сбора логов с фильтрацией через `labelFilter`,
где оператор `NotRegex` исключает строки, соответствующие заданному регулярному выражению.

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

Пример конфигурации для сбора и фильтрации событий аудита, связанных с работой kubelet,
хранящихся в файле `/var/log/kube-audit/audit.log`.
Фильтрация выполняется с помощью `logFilter`, который ищет в поле `userAgent` записи,
соответствующие регулярному выражению `"kubelet.*"`.

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

### Системные логи DKP

Пример конфигурации для сбора системных логов DKP, находящихся в файле `/var/log/syslog`.
Фильтрация сообщений с помощью `labelFilter` позволяет выделить только те записи,
которые относятся к следующим компонентам:
`d8-kubelet-forker`, `containerd`, `bashible` и `kernel`.

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

{% alert level="info" %}
Если вам нужны логи только одного пода или небольшой группы подов,
используйте [`kubernetesPods`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-kubernetespods), чтобы ограничить область сбора.
Фильтры следует применять только для тонкой настройки.
{%- endalert %}

## Буферизация логов

Использование буферизации повышает надежность и производительность системы сбора логов.
Буферизация может быть полезна в следующих случаях:

- **Временные перебои с подключением**.
  Если есть временные перебои или нестабильность соединения с системой хранения логов (например, с Elasticsearch),
  буфер позволяет временно сохранять логи и отправить их, когда соединение восстановится.

- **Сглаживание пиков нагрузки**.
  При внезапных всплесках объёма логов буфер позволяет сгладить пиковую нагрузку на систему хранения,
  предотвращая её перегрузку и потенциальную потерю данных.

- **Оптимизация производительности**.
  Буферизация помогает оптимизировать производительность системы сбора логов за счёт накопления логов и отправки их группами,
  что снижает количество сетевых запросов и улучшает общую пропускную способность.

За настройку буферизации отвечает [параметр `buffer`](/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-buffer) ресурса ClusterLogDestination.

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

## Отладка и расширенные возможности

### Включение debug-логов агента log-shipper

Чтобы включить debug-логи [агента `log-shipper`](/modules/log-shipper/) на узлах с информацией об HTTP-запросах, переиспользовании подключения,
трассировке и прочими данными, включите [параметр `debug`](/modules/log-shipper/configuration.html#parameters-debug) в конфигурации модуля `log-shipper`.

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: log-shipper
spec:
  version: 1
  enabled: true
  settings:
    debug: true
```

### Дополнительная информация о каналах передачи логов

Используя команды для Vector, можно получить дополнительную информацию о каналах передачи данных.

Для начала подключитесь к одному из подов `log-shipper`:

```bash
d8 k -n d8-log-shipper get pods -o wide | grep $node
d8 k -n d8-log-shipper exec $pod -it -c vector -- bash
```

Выполняйте последующие команды из командной оболочки пода.

#### Обзор топологии

Чтобы получить схему топологии вашей конфигурации:

1. Выполните команду `vector graph`. Будет сформирована схема в формате DOT.
1. Используйте [WebGraphviz](https://www.webgraphviz.com/) или аналогичный сервис для отрисовки схемы на основе содержимого DOT-файла.

Пример схемы для одного канала передачи логов в формате ASCII:

```text
+------------------------------------------------+
|  d8_cluster_source_flant-integration-d8-logs   |
+------------------------------------------------+
  |
  |
  v
+------------------------------------------------+
|       d8_tf_flant-integration-d8-logs_0        |
+------------------------------------------------+
  |
  |
  v
+------------------------------------------------+
|       d8_tf_flant-integration-d8-logs_1        |
+------------------------------------------------+
  |
  |
  v
+------------------------------------------------+
| d8_cluster_sink_flant-integration-loki-storage |
+------------------------------------------------+
```

#### Мониторинг нагрузки на каналы

Чтобы посмотреть объем трафика на каждом этапе обработки логов, используйте команду `vector top`.

Пример вывода команды:

![Vector TOP output](../../../images/log-shipper/vector_top.png)

#### Получение необработанных и промежуточных логов

Для просмотра входных данных на разных стадиях обработки логов используйте команду `vector tap`.
Указав в ней ID конкретного этапа обработки, вы сможете увидеть логи которые поступают на этом этапе.
Также поддерживаются выборки в формате glob, например, `cluster_logging_config/*`.

Примеры:

- Просмотр логов до применения правил трансформаций
  (`cluster_logging_config/*` является первой стадией обработки согласно выводу команды `vector graph`):

  ```bash
  vector tap 'cluster_logging_config/*'
  ```

- Изменённые логи, поступающие на вход следующих в цепочке компонентов каналов:

  ```bash
  vector tap 'transform/*'
  ```

#### Отладка VRL-правил

Для отладки правил [на языке Vector Remap Language (VRL)](https://vector.dev/docs/reference/vrl/)
используйте команду `vector vrl`.

Пример VRL-программы:

```text
. = {"test1": "lynx", "test2": "fox"}
del(.test2)
.
```

### Добавление поддержки нового source или sink

[Модуль `log-shipper`](/modules/log-shipper/) в DKP собирается на основе Vector с ограниченным набором [cargo-функций](https://doc.rust-lang.org/cargo/reference/features.html),
чтобы минимизировать размер запускаемого файла и ускорить сборку.

Чтобы посмотреть весь список поддерживаемых функций, выполните команду `vector list`.

Если нужный source или sink отсутствует, добавьте соответствующую cargo-функцию в Dockerfile.

## Особые случаи

### Сбор логов с продуктовых пространств имён через опцию labelSelector

Если в вашем кластере пространства имён размечены с помощью лейблов (например, `environment=production`),
вы можете использовать [опцию `labelSelector`](/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha2-spec-kubernetespods-labelselector) для сбора логов из продуктивных пространств имён.

Пример конфигурации:

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

### Лейбл для исключения подов и пространств имён

В DKP предусмотрен лейбл `log-shipper.deckhouse.io/exclude=true` для исключения определенных подов и пространств имён.
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
