---
title: "Расширенная конфигурация"
description: Примеры более глубокого использования модуля runtime-audit-engine Deckhouse.
---


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

Для получения метрик можно использовать PromQL-запрос `falcosecurity_falcosidekick_falco_events_total{}`:

```shell
d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" | jq
```

В будущем мы добавим Grafana dashboard для просмотра метрик.

## Эмуляция события Falco

Вы можете использовать утилиту event-generator для генерации событий Falco.

`event-generator` может генерировать различные подозрительные действия (syscalls, k8s audit events и др.).

Вы можете использовать следующую команду для запуска тестового набора событий в кластере Kubernetes:

```shell
d8 k run falco-event-generator --image=falcosecurity/event-generator run
```

## Эмуляция события Falcosidekick

Вы можете использовать Falcosidekick `/test` HTTP endpoint для отправки тестового события.

- Создайте отладочное событие, выполнив команду:

  ```shell
  nsenter -t $(pidof falcosidekick) curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" http://localhost:2801/test
  ```

- Проверьте метрику отладочного события:

  ```shell
  d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
    curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" \
    | jq '.data.result.[] | select (.metric.priority_raw == "debug")'
  ```

- Пример вывода:

  ```json
  {
    "metric": {
      "__name__": "falcosecurity_falcosidekick_falco_events_total",
      "container": "kube-rbac-proxy",
      "hostname": "falcosidekick",
      "instance": "192.168.208.7:4212",
      "job": "runtime-audit-engine",
      "node": "dev-master-0",
      "priority": "1",
      "priority_raw": "debug",
      "rule": "Test rule",
      "source": "internal",
      "tier": "cluster"
    },
    "value": [
      1744234729.799,
      "1"
    ]
  }
  ```
