---
title: "Модуль log-shipper: примеры конфигурации"
---

## Чтение логов из всех pod-ов кластера и направление их в Loki
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
    healthcheck:
      enabled: true
    encoding:
      codec: json
```

## Чтение логов подов из указанного namespace  с указанным label и перенаправление одновременно в Loki и Elasticsearch

Чтение логов подов из namespace `whispers` только с label `app=booking` и перенаправление одновременно в Loki и Elasticsearch:

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
    healthcheck:
      enabled: true
    encoding:
      codec: json
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

## Создание source в namespace и чтение логов всех pod-ов в этом NS, с направлением их в Loki

Следующий pipeline создает source в namespace: `test-whispers` и читает логи всех pod-ов в этом NS, пишет их в Loki:
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
    healthcheck:
      enabled: true
    encoding:
      codec: json
```

## Чтение только pod-ов в указанном namespace и имеющих определенный label
Пример чтения только pod-ов имеющих label `app=booking` в namespace `test-whispers`:
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
```
