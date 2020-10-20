---
title: "Модуль prometheus-metrics-adapter: Custom resources"
search: autoscaler, HorizontalPodAutoscaler 
---

{% capture cr_spec %}
* `.metadata.name` — имя метрики, используется в HPA.
* `.spec.query` — кастомный PromQL-запрос, который возвращает однозначное значение для вашего набора лейблов (используйте группировку операторами `sum() by()`, `max() by()` и пр.). В запросе необходимо **обязательно использовать** ключики:
    * `<<.LabelMatchers>>` — заменится на набор лейблов `{namespace="mynamespace",ingress="myingress"}`. Можно добавить свои лейблы через запятую как в [примере ниже](#пример-использования-кастомных-метрик-с-размером-очереди-rabbitmq).
    * `<<.GroupBy>>` — заменится на перечисление лейблов `namespace,ingress` для группировки (`max() by(...)`, `sum() by (...)` и пр.).
{% endcapture %}

Настройка ванильного prometheus-metrics-adapter-а — это достаточно трудоёмкий процесс и мы его несколько упростили, определив набор **CRD** с разным Scope

С помощью Cluster-ресурса можно определить метрику глобально, а с помощью Namespaced-ресурса можно её локально переопределять. Формат у всех CR одинаковый.

## Namespaced Custom resources
### `ServiceMetric`
{{ cr_spec }}
#### Пример

##### Пример использования кастомных метрик с размером очереди RabbitMQ

Имеем очередь "send_forum_message" в RabbitMQ, для которого зарегистрирован сервис "rmq". Если сообщений в очереди больше 42 — скейлимся.

{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}) by (<<.GroupBy>>)
```
{% endraw %}

### `IngressMetric`
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressMetric
metadata:
  name: mymetric
  namespace: mynamespace
spec:
  query: sum(ingress_nginx_detail_requests_total{<<.LabelMatchers>>}) by (<<.GroupBy>>)
```
{% endraw %}

### `PodMetric`
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
```
{% endraw %}

### `DeploymentMetric`
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeploymentMetric
metadata:
  name: my-deployment-metrics
spec:
  query: PromQL-запрос
```
{% endraw %}

### `StatefulsetMetric`
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: StatefulsetMetric
metadata:
  name: my-statefulset-metrics
spec:
  query: PromQL-запрос
```
{% endraw %}

### `NamespaceMetric`
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NamespaceMetric
metadata:
  name: my-namespace-metrics
spec:
  query: avg by (<<.GroupBy>>) (rails_puma_max_threads_total{<<.LabelMatchers>>,pod_name=~"madison-events-api-custom.*"}) - avg by (<<.GroupBy>>) (rails_puma_thread_pool_capacity_total{<<.LabelMatchers>>,pod_name=~"madison-events-api-custom.*"})
```
{% endraw %}

### `DaemonsetMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DaemonsetMetric
metadata:
  name: my-daemonset-metrics
spec:
  query: PromQL-запрос
```
{% endraw %}

## Cluster Custom resources

### `ClusterServiceMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}) by (<<.GroupBy>>)
```
{% endraw %}

### `ClusterIngressMetric` (не доступен пользователям)
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
{{ cr_spec }}
#### Пример
kind: ClusterIngressMetric
metadata:
  name: mymetric
spec:
  query: sum(ingress_nginx_detail_sent_bytes_sum{<<.LabelMatchers>>}) by (<<.GroupBy>>)
```
{% endraw %}

### `ClusterPodMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример
{% raw %}
```
apiVersion: deckhouse.io/v1alpha1
kind: ClusterPodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
```
{% endraw %}

### `ClusterDeploymentMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример

### `ClusterStatefulsetMetric` (не доступен пользователям)
{{ cr_spec }}
#### Пример

### `ClusterDaemonsetMetric` (не доступен пользователям)
{{ cr_spec }}

## PrometheusRule

Настраивать скейлинг по внешним метрикам (добавлять PrometheusRule в namespace d8-monitoring) корректно ТОЛЬКО из некоторого инфраструктурного репозитария, но не из репозиториев приложений. Всячески старайтесь вместо этого использовать, например, NamespaceMetric.

### Пример

{% raw %}
```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: prometheus-metrics-adapter-mymetric # Рекомендованный шаблон для названия вашего PrometheusRule.
  namespace: d8-monitoring # Важно!
  labels:
    prometheus: main # Важно!
    component: rules # Важно!
spec:
  groups:
  - name: prometheus-metrics-adapter.mymetric # Рекомендованный шаблон.
    rules:
    - record: kube_adapter_metric_mymetric # Как ваша новая метрика будет называться
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress) # Запрос, результаты которого попадут в итоговую метрику, нет смысла тащить в неё лишние лейблы.
```
{% endraw %}
