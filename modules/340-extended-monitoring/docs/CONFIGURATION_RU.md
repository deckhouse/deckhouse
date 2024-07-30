---
title: "Модуль extended-monitoring: настройки"
force_searchable: true
---

<!-- SCHEMA -->

## Как использовать `extended-monitoring-exporter`

Чтобы включить экспортирование extended-monitoring метрик, нужно навесить на namespace лейбл `extended-monitoring.deckhouse.io/enabled` любым удобным способом, например:
- добавить в проект соответствующий helm-чарт (рекомендуемый);
- добавить в описание `.gitlab-ci.yml` (kubectl patch/create);
- поставить руками (`kubectl label namespace my-app-production extended-monitoring.deckhouse.io/enabled=""`);
- настроить через [namespace-configurator](/documentation/v1/modules/600-namespace-configurator/) модуль.

Сразу же после этого для всех поддерживаемых Kubernetes-объектов в данном namespace в Prometheus появятся default-метрики + любые кастомные с префиксом `threshold.extended-monitoring.deckhouse.io/`. Для ряда [non-namespaced](#non-namespaced-kubernetes-объекты) Kubernetes-объектов, описанных ниже, мониторинг включается автоматически.

К Kubernetes-объектам `threshold.extended-monitoring.deckhouse.io/что-то свое` можно добавить любые другие лейблы с указанным значением. Пример: `kubectl label pod test threshold.extended-monitoring.deckhouse.io/disk-inodes-warning=30`.
В таком случае значение из лейбла заменит значение по умолчанию.

Слежение за объектом можно отключить индивидуально, поставив на него лейбл `extended-monitoring.deckhouse.io/enabled=false`. Соответственно, отключатся и лейблы по умолчанию, а также все алерты, привязанные к лейблам.

### Стандартные лейблы и поддерживаемые Kubernetes-объекты

Далее приведен список используемых в Prometheus Rules лейблов, а также их стандартные значения.

**Обратите внимание,** что все лейблы начинаются с префикса `threshold.extended-monitoring.deckhouse.io/`. Указанное в лейбле значение — число, которое устанавливает порог срабатывания алерта.

Например, лейбл `threshold.extended-monitoring.deckhouse.io/5xx-warning: "5"` на Ingress-ресурсе изменяет порог срабатывания алерта с 10% (по умолчанию) на 5%.

#### Non-namespaced Kubernetes-объекты

Non-namespaced Kubernetes-объекты не нуждаются в лейблах на namespace и мониторинг на них включается по умолчанию при включении модуля.

##### Node

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 70             |
| disk-bytes-critical                     | int (percent) | 80             |
| disk-inodes-warning                     | int (percent) | 90             |
| disk-inodes-critical                    | int (percent) | 95             |
| load-average-per-core-warning           | int           | 3              |
| load-average-per-core-critical          | int           | 10             |

> **Важно!** Эти лейблы **не** действуют для тех разделов, в которых расположены `imagefs` (по умолчанию — `/var/lib/docker`) и `nodefs` (по умолчанию — `/var/lib/kubelet`).
Для этих разделов пороги настраиваются полностью автоматически согласно [eviction thresholds в kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).
Значения по умолчанию см. [тут](https://github.com/kubernetes/kubernetes/blob/743e4fba6339237cc8d5c11413f76ea54b4cc3e8/pkg/kubelet/apis/config/v1beta1/defaults_linux.go#L22-L27), подробнее см. [экспортер](https://github.com/deckhouse/deckhouse/blob/main/modules/340-monitoring-kubernetes/images/kubelet-eviction-thresholds-exporter/).

#### Namespaced Kubernetes-объекты

##### Под

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 85             |
| disk-bytes-critical                     | int (percent) | 95             |
| disk-inodes-warning                     | int (percent) | 85             |
| disk-inodes-critical                    | int (percent) | 90             |

##### Ingress

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning            | int (percent) | 10            |
| 5xx-critical           | int (percent) | 20            |

##### Deployment

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable). Сработает, если недоступно реплик больше на указанное значение, чем разрешено в `maxUnavailable`. То есть при нуле сработает, если недоступно больше, чем указано в `maxUnavailable`, а при единице сработает, если недоступно больше, чем указано в `maxUnavailable`, плюс 1. Таким образом, у конкретных Deployment, которые находятся в namespace со включенным расширенным мониторингом и которым допустимо быть недоступными, можно подкрутить этот параметр, чтобы не получать ненужные алерты.

##### StatefulSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

##### DaemonSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** [maxUnavailable](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#max-unavailable) (см. комментарии к [Deployment](#deployment)).

##### CronJob

Работает только выключение через лейбл `extended-monitoring.deckhouse.io/enabled=false`.

### Как работает

Модуль экспортирует в Prometheus специальные лейблы Kubernetes-объектов. Позволяет улучшить Prometheus-правила путем добавления порога срабатывания для алертов.
Использование метрик, экспортируемых данным модулем, позволяет, например, заменить «магические» константы в правилах.

До:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> 1
```

После:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> on (namespace, statefulset)
(
  max by (namespace, statefulset) (extended_monitoring_statefulset_threshold{threshold="replicas-not-ready"})
)
```
