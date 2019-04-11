Модуль extended-monitoring
==========================

## Назначение

Добавляет [алерты](#non-namespaced-kubernetes-objects) по месту и inode на нодах, плюс включает «расширенный мониторинг» объектов в [указанных](#конфигурация) `namespace`, с возможностью кастомизации порогов алертов (очень просто включается и рекомендуется для включения как минимум на продуктивных контурах).

## Конфигурация

Чтобы включить экспортирование extended-monitoring метрик, нужно навесить на Namespace аннотацию `extended-monitoring.flant.com/enabled` любым удобным способом, например:
- добавить в проект соответствующий helm-чарт (рекомендуемый)
- добавить в описание `.gitlab-ci.yml` (kubectl patch/create)
- поставить руками (`kubectl annotate namespace my-app-production extended-monitoring.flant.com/enabled=""`).

Сразу же после этого, для всех поддерживаемых Kubernetes объектов в данном Namespace в Prometheus появятся default метрики + любые кастомные с префиксом `threshold.extended-monitoring.flant.com/`. Для ряда [non-namespaced](#non-namespaced-kubernetes-objects) Kubernetes объектов, описанных ниже, мониторинг и стандартные аннотации включаются автоматически.

К Kubernetes объектам `threshold.extended-monitoring.flant.com/что-то своё` можно добавить любые другие аннотации с указанным значением. Пример: `kubectl annotate pod test monitoring.flant.com/disk-inodes-warning-threshold=30`.
В таком случае, значение из аннотации заменит значение по-умолчанию.

## Стандартные аннотации и поддерживаемые Kubernetes объекты

Далее приведён список используемых в Prometheus Rules аннотаций, а также их стандартные значения.

**Внимание!** Все аннотации:
1. Начинаются с префикса `threshold.extended-monitoring.flant.com/`;
2. Имеют целочисленное значение в качестве value, за исключением Namespace аннотации `extended-monitoring.flant.com/enabled` (в которой value можно опустить). Указанное в value значение устанавливает порог срабатывания алерта.

### Non-namespaced Kubernetes objects

Не нуждаются в аннотации на Namespace. Включены по-умолчанию.

#### Node

| Annotation                              | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning            | int (percent) | 70             |
| disk-bytes-critical           | int (percent) | 80             |
| disk-inodes-warning           | int (percent) | 85             |
| disk-inodes-critical          | int (percent) | 90             |

> ВНИМАНИЕ! Алерты по диску пока не работают с Rook ([Пруф](https://flant.slack.com/archives/CFGTVF1KJ/p1554192138002900)).

### Namespaced Kubernetes objects

#### Pod

| Annotation                              | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning            | int (percent) | 85             |
| disk-bytes-critical           | int (percent) | 95             |
| disk-inodes-warning           | int (percent) | 85             |
| disk-inodes-critical          | int (percent) | 90             |
| container-throttling-warning  | int (percent) | 25             |
| container-throttling-critical | int (percent) | 50             |

> ВНИМАНИЕ! Алерты по диску пока не работают с Rook ([Пруф](https://flant.slack.com/archives/CFGTVF1KJ/p1554192138002900)).

#### Ingress

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning  | int (percent) | 10            |
| 5xx-critical | int (percent) | 20            |

#### Deployment

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready  | int (count) | 0            |

Порог подразумевает количество недоступных реплик **СВЕРХ** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable). Сработает, если недоступно реплик больше на указанное значение чем разрешено в `maxUnavailable` - т.е. при нуле сработает, если недоступно больше чем указано в `maxUnavailable`, а при единице, сработает если недоступно больше чем указано в `maxUnavailable` плюс 1. Таким образом можно у конкретных Deployment, находящихся в namespace со включенным расширенным мониторингом, и которым можно быть недоступными, подкрутить этот параметр, чтобы не получать ненужные алерты.

#### Statefulset

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready  | int (count) | 0            |

Порог подразумевает количество недоступных реплик **СВЕРХ** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

#### DaemonSet

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready  | int (count) | 0            |

Порог подразумевает количество недоступных реплик **СВЕРХ** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

## Как работает

Модуль экспортирует в Prometheus специальные аннотации Kubernetes объектов. Позволяет улучшить Prometheus правила, путём добавления порога срабатывания для алертов. Использование метрик, экспортируемых данным модулем, позволяет, например, заменить "магические" константы в правилах.

Правила, добавляемые в Prometheus этим модулем, лежат [здесь](modules/350-extended-monitoring/prometheus-rules).

До:
```
max by (namespace, pod, container) (
  (
    rate(container_cpu_cfs_throttled_periods_total[5m])
    /
    rate(container_cpu_cfs_periods_total[5m])
  )
  > 0.85
)
```

После:
```
max by (namespace, pod, container) (
  (
    rate(container_cpu_cfs_throttled_periods_total[5m])
    /
    rate(container_cpu_cfs_periods_total[5m])
  )
  > on (namespace, pod) group_left
    max by (namespace, pod) (extended_monitoring_pod_threshold{threshold="container-throttling-critical"}) / 100
)
```

### Development

Информацию о разработке можно получить в [DEVELOPMENT.md](modules/350-extended-monitoring/DEVELOPMENT.md).
