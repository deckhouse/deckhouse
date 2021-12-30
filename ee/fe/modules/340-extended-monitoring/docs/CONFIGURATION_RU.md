---
title: "Модуль extended-monitoring: настройки"
---

## Параметры

<!-- SCHEMA -->

## Как использовать `extended-monitoring-exporter`

Чтобы включить экспортирование extended-monitoring метрик, нужно навесить на Namespace аннотацию `extended-monitoring.flant.com/enabled` любым удобным способом, например:
- добавить в проект соответствующий helm-чарт (рекомендуемый)
- добавить в описание `.gitlab-ci.yml` (kubectl patch/create)
- поставить руками (`kubectl annotate namespace my-app-production extended-monitoring.flant.com/enabled=""`).
- настроить через [namespace-configurator](/ru/documentation/v1/modules/600-namespace-configurator/) модуль.

Сразу же после этого, для всех поддерживаемых Kubernetes объектов в данном Namespace в Prometheus появятся default метрики + любые кастомные с префиксом `threshold.extended-monitoring.flant.com/`. Для ряда [non-namespaced](#non-namespaced-kubernetes-objects) Kubernetes объектов, описанных ниже, мониторинг и стандартные аннотации включаются автоматически.

К Kubernetes объектам `threshold.extended-monitoring.flant.com/что-то своё` можно добавить любые другие аннотации с указанным значением. Пример: `kubectl annotate pod test threshold.extended-monitoring.flant.com/disk-inodes-warning-threshold=30`.
В таком случае, значение из аннотации заменит значение по умолчанию.

Слежение за объектом можно отключить индивидуально, поставив на него аннотацию `extended-monitoring.flant.com/enabled=false`. Соответственно, отключатся и аннотации по умолчанию, а также все алерты, привязанные к аннотациям.

### Стандартные аннотации и поддерживаемые Kubernetes объекты

Далее приведён список используемых в Prometheus Rules аннотаций, а также их стандартные значения.

**Внимание!** Все аннотации:
1. Начинаются с префикса `threshold.extended-monitoring.flant.com/`;
2. Имеют целочисленное значение в качестве value, за исключением Namespace-аннотации `extended-monitoring.flant.com/enabled` (в которой value можно опустить). Указанное в value значение устанавливает порог срабатывания алерта.

#### Non-namespaced Kubernetes objects

Не нуждаются в аннотации на Namespace. Включены по умолчанию.

##### Node

| Annotation                              | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 70             |
| disk-bytes-critical                     | int (percent) | 80             |
| disk-inodes-warning                     | int (percent) | 85             |
| disk-inodes-critical                    | int (percent) | 90             |
| load-average-per-core-warning           | int           | 3              |
| load-average-per-core-critical          | int           | 10             |

> ВНИМАНИЕ! Эти аннотации **не** действуют для тех разделов, в которых расположены `imagefs` (по умолчанию, — `/var/lib/docker`) и `nodefs` (по умолчанию, — `/var/lib/kubelet`).
Для этих разделов пороги настраиваются полностью автоматически согласно [eviction thresholds в kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).
Значения по умолчанию см. [тут](https://github.com/kubernetes/kubernetes/blob/743e4fba6339237cc8d5c11413f76ea54b4cc3e8/pkg/kubelet/apis/config/v1beta1/defaults_linux.go#L22-L27), подробнее см. [экспортер](https://github.com/deckhouse/deckhouse/blob/main/modules/340-monitoring-kubernetes/images/kubelet-eviction-thresholds-exporter/loop).

#### Namespaced Kubernetes objects

##### Pod

| Annotation                              | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 85             |
| disk-bytes-critical                     | int (percent) | 95             |
| disk-inodes-warning                     | int (percent) | 85             |
| disk-inodes-critical                    | int (percent) | 90             |
| container-throttling-warning            | int (percent) | 25             |
| container-throttling-critical           | int (percent) | 50             |
| container-cores-throttling-warning      | int (cores)   |                |
| container-cores-throttling-critical     | int (cores)   |                |

##### Ingress

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning            | int (percent) | 10            |
| 5xx-critical           | int (percent) | 20            |

##### Deployment

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable). Сработает, если недоступно реплик больше на указанное значение чем разрешено в `maxUnavailable`. Т.е. при нуле сработает, если недоступно больше чем указано в `maxUnavailable`, а при единице, сработает если недоступно больше чем указано в `maxUnavailable` плюс 1. Таким образом, у конкретных Deployment, находящихся в Namespace со включенным расширенным мониторингом и которым допустимо быть недоступными, можно подкрутить этот параметр, чтобы не получать ненужные алерты.

##### Statefulset

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

##### DaemonSet

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

##### CronJob

Работает только выключение через аннотацию `extended-monitoring.flant.com/enabled=false`.

### Как работает

Модуль экспортирует в Prometheus специальные аннотации Kubernetes-объектов. Позволяет улучшить Prometheus-правила, путём добавления порога срабатывания для алертов. 
Использование метрик, экспортируемых данным модулем, позволяет, например, заменить "магические" константы в правилах.

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
