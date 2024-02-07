---
title: "Модуль flow-schema: FAQ"
---

## Как проверить состояние priority level'ов?

Выполните:

```shell
kubectl get --raw /debug/api_priority_and_fairness/dump_priority_levels
```

## Как проверить состояние очередей priority level'ов?

Выполните:

```shell
kubectl get --raw /debug/api_priority_and_fairness/dump_queues
```

## Полезные метрики

- `apiserver_flowcontrol_rejected_requests_total` — общее число отброшенных запросов.
- `apiserver_flowcontrol_dispatched_requests_total` — общее число обработанных запросов.
- `apiserver_flowcontrol_current_inqueue_requests` — количество запросов в очередях.
- `apiserver_flowcontrol_current_executing_requests` — количество запросов в обработке.
