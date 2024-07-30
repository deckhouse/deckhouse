---
title: "Модуль runtime-audit-engine: расширенная конфигурация"
description: Примеры более глубокого использования модуля runtime-audit-engine Deckhouse.
---

{% raw %}

## Включение логов для отладки

### Falco

По умолчанию используется уровень логирования `debug`.

### Falcosidekick

По умолчанию отладочное логирование выключено в `Falcosidekick`.

Для включения отладочного логирования установите параметр `spec.settings.debugLogging` в `true`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: runtime-audit-engine
spec:
  enabled: true
  settings:
    debugLogging: true
```

## Просмотр метрик

Для получения метрик можно использовать PromQL-запрос `falco_events{}`:

```shell
kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
```

В будущем мы добавим Grafana dashboard для просмотра метрик.

## Эмуляция события Falco

Вы можете использовать утилиту [event-generator](https://github.com/falcosecurity/event-generator) для генерации событий Falco.

`event-generator` может генерировать различные подозрительные действия (syscalls, k8s audit events и др.).

Вы можете использовать следующую команду для запуска тестового набора событий в кластере Kubernetes:

```shell
kubectl run falco-event-generator --image=falcosecurity/event-generator run
```

Если вам нужно реализовать действие, воспользуйтесь [руководством](https://github.com/falcosecurity/event-generator/blob/main/events/README.md).

## Эмуляция события Falcosidekick

Вы можете использовать [Falcosidekick](https://github.com/falcosecurity/falcosidekick) `/test` HTTP endpoint для отправки тестового события во все включенные выходы.

- Получите список подов в пространстве имен `d8-runtime-audit-engine`:

  ```shell
  kubectl -n d8-runtime-audit-engine get pods
  ```

  Пример вывода:

  ```text
  NAME                         READY   STATUS    RESTARTS   AGE
  runtime-audit-engine-4cpjc   4/4     Running   0          3d12h
  runtime-audit-engine-rn7nj   4/4     Running   0          3d12h
  ```

- Получите IP-адрес пода `runtime-audit-engine-4cpjc`:

  ```shell
  export POD_IP=$(kubectl -n d8-runtime-audit-engine get pod runtime-audit-engine-4cpjc --template '{{.status.podIP}}')
  ```

- Создайте отладочное событие, выполнив запрос:

  ```shell
  kubectl run curl --image=curlimages/curl curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" $POD_IP:2801/test
  ```

- Проверьте метрику отладочного события:

  ```shell
  kubectl -n d8-monitoring exec -it prometheus-main-0 prometheus --  \
    curl -s http://127.0.0.1:9090/api/v1/query\?query\=falco_events | jq
  ```

- Пример вывода:

  ```json
  {
    "metric": {
      "__name__": "falco_events",
      "container": "kube-rbac-proxy",
      "instance": "192.168.199.60:4212",
      "job": "runtime-audit-engine",
      "node": "dev-master-0",
      "priority": "Debug",
      "rule": "Test rule",
      "tier": "cluster"
    },
    "value": [
      1687150913.828,
      "2"
    ]
  }
  ```

{% endraw %}
