---
title: "Модуль prometheus-metrics-adapter: Custom resources"
search: autoscaler, HorizontalPodAutoscaler 
---

{% capture cr_spec %}

* `.metadata.name` — имя метрики, используется в HPA.
* `.spec.query` — кастомный PromQL-запрос, который возвращает однозначное значение для вашего набора лейблов (используйте группировку операторами `sum() by()`, `max() by()` и пр.). В запросе необходимо **обязательно использовать** ключи:
  * `<<.LabelMatchers>>` — заменится на набор лейблов `{namespace="mynamespace"###PLACEHOLDER###}`. Можно добавить собственные лейблы через запятую ([пример](usage.html#пример-использования-кастомных-метрик-с-размером-очереди-rabbitmq)).
  * `<<.GroupBy>>` — заменится на перечисление лейблов `namespace###PLACEHOLDER2###` для группировки (`max() by(...)`, `sum() by (...)` и пр.).
{% endcapture %}

Настройка ванильного `prometheus-metrics-adapter` — трудоемкий процесс. Мы его упростили, определив набор **CustomResourceDefinition** с разной областью видимости (scope).

С помощью cluster-wide-ресурса можно определить метрику глобально, а с помощью namespaced-ресурса ее можно локально переопределять. Формат всех кастомных ресурсов — одинаковый.

## Namespaced custom resources

### `ServiceMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',service="myservice"'  | replace: '###PLACEHOLDER2###', ',service' }}

### `IngressMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',ingress="myingress"' | replace: '###PLACEHOLDER2###', ',ingress' }}

### `PodMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',pod="mypod-xxxxx"' | replace: '###PLACEHOLDER2###', ',pod' }}

### `DeploymentMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',deployment="mydeployment"' | replace: '###PLACEHOLDER2###', ',deployment' }}

### `StatefulSetMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',statefulset="mystatefulset"' | replace: '###PLACEHOLDER2###', ',statefulset' }}

### `NamespaceMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ''  | replace: '###PLACEHOLDER2###', '' }}

### `DaemonSetMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',daemonset="mydaemonset"' | replace: '###PLACEHOLDER2###', ',daemonset' }}

## Cluster custom resources

### `ClusterServiceMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',service="myservice"'  | replace: '###PLACEHOLDER2###', ',service' }}

### `ClusterIngressMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',ingress="myingress"' | replace: '###PLACEHOLDER2###', ',ingress' }}

### `ClusterPodMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',pod="mypod-xxxxx"' | replace: '###PLACEHOLDER2###', ',pod' }}

### `ClusterDeploymentMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',deployment="mydeployment"' | replace: '###PLACEHOLDER2###', ',deployment' }}

### `ClusterStatefulSetMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',statefulset="mystatefulset"' | replace: '###PLACEHOLDER2###', ',statefulset' }}

### `ClusterDaemonSetMetric` (недоступен пользователям)

{{ cr_spec | replace: '###PLACEHOLDER###', ',daemonset="mydaemonset"' | replace: '###PLACEHOLDER2###', ',daemonset' }}
