---
title: "Модуль prometheus-metrics-adapter: Custom resources"
search: autoscaler, HorizontalPodAutoscaler 
---

{% capture cr_spec %}
* `.metadata.name` — имя метрики, используется в HPA.
* `.spec.query` — кастомный PromQL-запрос, который возвращает однозначное значение для вашего набора лейблов (используйте группировку операторами `sum() by()`, `max() by()` и пр.). В запросе необходимо **обязательно использовать** ключики:
    * `<<.LabelMatchers>>` — заменится на набор лейблов `{namespace="mynamespace",ingress="myingress"}`. Можно добавить свои лейблы через запятую как в [примере ниже](usage.html#пример-использования-кастомных-метрик-с-размером-очереди-rabbitmq).
    * `<<.GroupBy>>` — заменится на перечисление лейблов `namespace,ingress` для группировки (`max() by(...)`, `sum() by (...)` и пр.).
{% endcapture %}

Настройка ванильного prometheus-metrics-adapter-а — это достаточно трудоёмкий процесс и мы его несколько упростили, определив набор **CRD** с разным Scope

С помощью Cluster-ресурса можно определить метрику глобально, а с помощью Namespaced-ресурса можно её локально переопределять. Формат у всех CR одинаковый.

## Namespaced Custom resources
### `ServiceMetric`
{{ cr_spec }}

### `IngressMetric`
{{ cr_spec }}

### `PodMetric`
{{ cr_spec }}

### `DeploymentMetric`
{{ cr_spec }}

### `StatefulSetMetric`
{{ cr_spec }}

### `NamespaceMetric`
{{ cr_spec }}

### `DaemonsetMetric` (не доступен пользователям)
{{ cr_spec }}

## Cluster Custom resources

### `ClusterServiceMetric` (не доступен пользователям)
{{ cr_spec }}

### `ClusterIngressMetric` (не доступен пользователям)
{{ cr_spec }}

### `ClusterPodMetric` (не доступен пользователям)
{{ cr_spec }}

### `ClusterDeploymentMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример

### `ClusterStatefulSetMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример

### `ClusterDaemonsetMetric` (не доступен пользователям)
{{ cr_spec }}

## PrometheusRule

* `.metadata.namespace` — всегда `d8-monitoring`;
* `.metadata.labels.prometheus — всегда  `main`;
* `.metadata.labels.component — всегда  `rules`;
* `.spec.groups:
    * `name` — имя, согласно рекомендованному шаблону (например `prometheus-metrics-adapter.mymetric`.
    * `rules`:
      * `record` — название метрики.
      * `expr` — запрос, результаты которого попадут в итоговую метрику.
