---
title: "Модуль log-shipper: трансформации логов"
description: Примеры использования трансформаций логов
---

{% raw %}

## Преобразования смешанных логов, JSON или строк к JSON. Парсинг JSON и уменьшение вложенности

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string to json
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: String
          string:
            targetField: msg
            depth: 1
```

```bash
# Логи:

/docker-entrypoint.sh: Configuration complete; ready for start up
{"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

# Результат преобразования:

"message": { "msg": "/docker-entrypoint.sh: Configuration complete; ready for start up"}
"message": {"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

```

## Преобразование смешанных логов, JSON или Klog к JSON. Парсинг JSON

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string to json
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: Klog
```

```bash
# Логи:

I0505 17:59:40.692994   28133 klog.go:70] hello from klog
{"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

# Результат преобразования:

"message": {"file":"klog.go","id":28133,"level":"info","line":70,"message":"hello from klog","timestamp":"2025-05-05T17:59:40.692994Z"}
"message": {"level" : "info","msg" : "fetching.module.release", "releasechannel" : "Stable", "time" : "2025-06-23T08:00:29Z"}

```

## Парсинг JSON и уменьшение вложенности

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string to json
  ...
spec:
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: JSON
          json:
            depth: 1
```

```bash
# Лог:

{"level" : { "severity": "info" },"msg" : "fetching.module.release"}

# Результат преобразования:

"message": {"level" : "{ \"severity\": \"info\" }","msg" : "fetching.module.release"}

```

## Замена точек на подчеркивания в ключах лейбла

- При применении трансформации к лейблам в message необходимо предварительно выполнить трансформацию esureStructuredMessage для парсинга json

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string to json
spec:
  ...
  transformations:
    - action: ReplaceDotKeys
      replaceDotKeys:
        labels:
          - pod_labels
```

```bash
# Лог:

{"msg" : "fetching.module.release"} # Лейбл пода pod.app=test

# Результат преобразования:

{"message": {"msg" : "fetching.module.release"}, pod_labels: {"pod_app": "test"}}

```

## Удаление лейблов

- При применении трансформации к лейблам в message необходимо предварительно выполнить трансформацию esureStructuredMessage для парсинга json

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string to json
spec:
  ...
  transformations:
    - action: DropLabels
      dropLabels:
        labels:
          - example
```

## Пример удаления лейбла из message

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingDestination
metadata:
  name: string to json
spec:
  ...
  transformations:
    - action: EnsureStructuredMessage
      ensureStructuredMessage:
        soureFormat: JSON
          json:
            depth: 2
    - action: DropLabels
      dropLabels:
        labels:
          - message.example
```

```bash
# Лог:

{"msg" : "fetching.module.release", "example": "test"}

# Результат преобразования:

"message": {"msg" : "fetching.module.release"}

```
