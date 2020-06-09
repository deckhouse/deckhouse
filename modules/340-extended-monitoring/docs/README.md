---
title: "Модуль extended-monitoring"
permalink: /modules/340-extended-monitoring/
sidebar: modules-extended-monitoring
hide_sidebar: false
---

Состоит из двух Prometheus exporter'ов:

1. `extended-monitoring-exporter` — генерирует метрики на основе аннотаций Kubernetes-объектов;
2. `image-availability-exporter` — генерирует метрики о проблемах доступа к Docker-образу в registry

## image-availability-exporter

### Назначение

Добавляет метрики и алерты, позволяющие узнать о проблемах доступа к образу, прописанному в поле `image` из Deployments, StatefulSets, DaemonSets, CronJobs, в registry.

### Конфигурация

* `imageAvailabilityExporter`
  * `ignoredImages` — список имён образов, наличие которых не надо проверять в registry. Например, `alpine:3.12` или `quay.io/test/test:v1.1`.
    * Формат — массив строк.
    * Опциональный параметр.
  * `skipRegistryCertVerification` — пропускать ли проверку сертификата container registry.
    * Формат — bool. По-умолчанию, `false`.

## extended-monitoring-exporter

### Назначение

Добавляет [алерты](#non-namespaced-kubernetes-objects) по месту и по inode на нодах, плюс включает «расширенный мониторинг» объектов в [указанных](#конфигурация) `namespace`, с возможностью кастомизации порогов алертов (очень просто включается и рекомендуется для включения как минимум на продуктивных контурах).

### Конфигурация

Чтобы включить экспортирование extended-monitoring метрик, нужно навесить на Namespace аннотацию `extended-monitoring.flant.com/enabled` любым удобным способом, например:
- добавить в проект соответствующий helm-чарт (рекомендуемый)
- добавить в описание `.gitlab-ci.yml` (kubectl patch/create)
- поставить руками (`kubectl annotate namespace my-app-production extended-monitoring.flant.com/enabled=""`).

Сразу же после этого, для всех поддерживаемых Kubernetes объектов в данном Namespace в Prometheus появятся default метрики + любые кастомные с префиксом `threshold.extended-monitoring.flant.com/`. Для ряда [non-namespaced](#non-namespaced-kubernetes-objects) Kubernetes объектов, описанных ниже, мониторинг и стандартные аннотации включаются автоматически.

К Kubernetes объектам `threshold.extended-monitoring.flant.com/что-то своё` можно добавить любые другие аннотации с указанным значением. Пример: `kubectl annotate pod test monitoring.flant.com/disk-inodes-warning-threshold=30`.
В таком случае, значение из аннотации заменит значение по-умолчанию.

Слежение за объектом можно отключить индивидуально, поставив на него аннотацию `extended-monitoring.flant.com/enabled=false`. Соответственно, отключатся и аннотации по-умолчанию, а также все алерты, привязанные к аннотациям.

### Стандартные аннотации и поддерживаемые Kubernetes объекты

Далее приведён список используемых в Prometheus Rules аннотаций, а также их стандартные значения.

**Внимание!** Все аннотации:
1. Начинаются с префикса `threshold.extended-monitoring.flant.com/`;
2. Имеют целочисленное значение в качестве value, за исключением Namespace аннотации `extended-monitoring.flant.com/enabled` (в которой value можно опустить). Указанное в value значение устанавливает порог срабатывания алерта.

#### Non-namespaced Kubernetes objects

Не нуждаются в аннотации на Namespace. Включены по-умолчанию.

##### Node

| Annotation                              | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning            | int (percent) | 70             |
| disk-bytes-critical           | int (percent) | 80             |
| disk-inodes-warning           | int (percent) | 85             |
| disk-inodes-critical          | int (percent) | 90             |
| load-average-per-core-warning | int  | 3            |
| load-average-per-core-critical | int  | 10             |

> ВНИМАНИЕ! Эти аннотации НЕ действуют для тех разделов, в которых расположены imagefs (по-умолчанию, /var/lib/docker) и nodefs (по-умолчанию, /var/lib/kubelet).
Для этих разделов пороги настраиваются полностью автоматически согласно [eviction thresholds в kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).
Значения по-умолчанию см. [тут](https://github.com/kubernetes/kubernetes/blob/743e4fba6339237cc8d5c11413f76ea54b4cc3e8/pkg/kubelet/apis/config/v1beta1/defaults_linux.go#L22-L27), подробнее см. [экспортер](modules/300-prometheus/images/kubelet-eviction-thresholds-exporter/loop) и [отдельные](modules/300-prometheus/prometheus-rules/kubernetes/eviction-bytes.yaml) [правила](modules/300-prometheus/prometheus-rules/kubernetes/eviction-inodes.yaml).

> ВНИМАНИЕ! Алерты по диску пока не работают с Rook ([Пруф](https://flant.slack.com/archives/CFGTVF1KJ/p1554192138002900)).

#### Namespaced Kubernetes objects

##### Pod

| Annotation                              | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning            | int (percent) | 85             |
| disk-bytes-critical           | int (percent) | 95             |
| disk-inodes-warning           | int (percent) | 85             |
| disk-inodes-critical          | int (percent) | 90             |
| container-throttling-warning  | int (percent) | 25             |
| container-throttling-critical | int (percent) | 50             |
| container-cores-throttling-warning  | int (cores) |              |
| container-cores-throttling-critical | int (cores) |              |
| container-restarts-1h         | int (count)   | 5              |
| container-restarts-24h        | int (count)   | 5              |

> ВНИМАНИЕ! Алерты по диску пока не работают с Rook ([Пруф](https://flant.slack.com/archives/CFGTVF1KJ/p1554192138002900)).

##### Ingress

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning  | int (percent) | 10            |
| 5xx-critical | int (percent) | 20            |

##### Deployment

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready  | int (count) | 0            |

Порог подразумевает количество недоступных реплик **СВЕРХ** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable). Сработает, если недоступно реплик больше на указанное значение чем разрешено в `maxUnavailable` - т.е. при нуле сработает, если недоступно больше чем указано в `maxUnavailable`, а при единице, сработает если недоступно больше чем указано в `maxUnavailable` плюс 1. Таким образом можно у конкретных Deployment, находящихся в namespace со включенным расширенным мониторингом, и которым можно быть недоступными, подкрутить этот параметр, чтобы не получать ненужные алерты.

##### Statefulset

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready  | int (count) | 0            |

Порог подразумевает количество недоступных реплик **СВЕРХ** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

##### DaemonSet

| Annotation             | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready  | int (count) | 0            |

Порог подразумевает количество недоступных реплик **СВЕРХ** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

##### CronJob

Работает только выключение через аннотацию `extended-monitoring.flant.com/enabled=false`.

### Как работает

Модуль экспортирует в Prometheus специальные аннотации Kubernetes объектов. Позволяет улучшить Prometheus правила, путём добавления порога срабатывания для алертов. Использование метрик, экспортируемых данным модулем, позволяет, например, заменить "магические" константы в правилах.

Правила, добавляемые в Prometheus этим модулем, лежат [здесь]({{ site.baseurl }}/modules/340-extended-monitoring/prometheus-rules).

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

#### Development

Информацию о разработке можно получить в [соответствующем разделе]({{ site.baseurl }}/modules/340-extended-monitoring/development.html).
