---
title: "Модуль prometheus-metrics-adapter: Custom resources"
search: autoscaler, HorizontalPodAutoscaler 
---

{% capture cr_spec %}
* `.metadata.name` — имя метрики, используется в HPA.
* `.spec.query` — кастомный PromQL-запрос, который возвращает однозначное значение для вашего набора label'ов (используйте группировку операторами `sum() by()`, `max() by()` и пр.). В запросе необходимо **обязательно использовать** ключи:
  * `<<.LabelMatchers>>` — заменится на набор label'ов `{namespace="mynamespace",ingress="myingress"}`. Можно добавить свои label'ы через запятую как в [примере ниже](usage.html#пример-использования-кастомных-метрик-с-размером-очереди-rabbitmq).
  * `<<.GroupBy>>` — заменится на перечисление label'ов `namespace,ingress` для группировки (`max() by(...)`, `sum() by (...)` и пр.).
{% endcapture %}

Настройка ванильного `prometheus-metrics-adapter` — это достаточно трудоёмкий процесс и мы его несколько упростили, определив набор **CRD** с разным Scope.

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

### `DaemonSetMetric` (недоступен пользователям)

{{ cr_spec }}

## Cluster Custom resources

### `ClusterServiceMetric` (недоступен пользователям)

{{ cr_spec }}

### `ClusterIngressMetric` (недоступен пользователям)

{{ cr_spec }}

### `ClusterPodMetric` (недоступен пользователям)

{{ cr_spec }}

### `ClusterDeploymentMetric` (недоступен пользователям)

{{ cr_spec }}

#### Пример

### `ClusterStatefulSetMetric` (недоступен пользователям)

{{ cr_spec }}

#### Пример

### `ClusterDaemonSetMetric` (недоступен пользователям)

{{ cr_spec }}
