---
title: "The prometheus-metrics-adapter module: usage"
search: autoscaler, HorizontalPodAutoscaler
---

Note that only HPA (Horizontal Pod Autoscaling) with [apiVersion: autoscaling/v2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#objectmetricsource-v2-autoscaling), whose support has been available since Kubernetes v1.12, is discussed below.

Configuring HPA requires:
* defining what is being scaled (`.spec.scaleTargetRef`);
* defining the scaling range (`.spec.minReplicas`, `.scale.maxReplicas`);
* defining metrics to be used as the basis for scaling (`.spec.metrics`) and registering them with the Kubernetes API.

Metrics in terms of HPA are of three types:
* [classic](#classic-scaling-by-custom-resource-consumption) — of type (`.spec.metrics[].type`) "Resource"; these are used for simple scaling based on CPU and memory consumption;
* [custom](#scaling-by-custom-metrics) — of type (`.spec.metrics[].type`) "Pods" or "Object";
* [external](#apply-external-metrics-to-hpa) — of type (`.spec.metrics[].type`) "External".



**Caution!** [By default,](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#default-behavior) HPA uses different approaches for scaling:
* If the metrics [indicate](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) that scaling **up** is required, it is done immediately (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 0). The only limitation is the rate of increase: pods can double in 15 seconds, but if there are less than 4 pods, 4 new pods will be added.
* If the metrics [indicate](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) that scaling **down** is required, it happens within 5 minutes (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 300): suggestions for a new number of replicas are calculated, then the largest value is selected. There is no limit on the number of pods to be removed at once.

If metrics are subject to fluctuations that result in a surge of unnecessary application replicas, the following approaches are used:
* Wrapping the metric with an aggregation function (e. g., `avg_over_time()`) if the metric is defined by a PromQL query. For more details, see. [example](#example-use-unstable-custom-metrics).
* Increasing the stabilization window (parameter `spec.behavior.scaleUp.stabilizationWindowSeconds`) in the _HorizontalPodAutoscaler_ resource. During the this period, requests to increase the number of replicas will be accumulated, then the most modest request will be selected. This method is identical to applying the `min_over_time(<stabilizationWindowSeconds>)` aggregation function, but only if the metric is increasing and scaling **up** is required. For scaling **down**, the default settings usually work good enough. For more details, see [example](#classical-scaling-by-resource-consumption).
* Limiting the rate of increase of the new replica count with `spec.behavior.scaleUp.policies`.

## Scaling types

Используйте следующие метрики для масштабирования приложений:
1. [Классического типа](#классическое-масштабирование-по-потреблению-ресурсов).
1. [Кастомные namespace-scoped-метрики](#масштабирование-по-кастомным-метрикам). При условии, если у вас одно приложение, источник метрик находится внутри namespace и связан с одним из объектов.
1. [Кастомные cluster-wide-метрики](#масштабирование-по-кастомным-метрикам). При условии, если у вас много приложений используют одинаковую метрику, источник которой находится в namespace приложения, и она связана с одним из объектов. Подобные метрики предусмотрены на случай необходимости выделения общих инфраструктурных компонентов в отдельный деплой («infra»).
1. Если источник метрики не привязан к namespace приложения, используйте [внешние](#применяем-внешние-метрики-в-hpa) метрики. Например, метрики облачного провайдера или внешнего SaaS-сервиса.

**Важно!** Рекомендуется использовать вариант 1 ([классические](#классическое-масштабирование-по-потреблению-ресурсов) метрики), или вариант 2 ([кастомные](#масштабирование-по-кастомным-метрикам) метрики, определяемые в _Namespace_). В этом случае, рекомендуется определить конфигурацию приложения, включающую его автоматическое масштабирование, в репозиторий самого приложения. Следует рассматривать варианты 3 и 4 только в том случае, если у вас имеется большая коллекция идентичных микросервисов.

## Классическое масштабирование по потреблению ресурсов

Пример HPA для масштабирования по базовым метрикам из `metrics.k8s.io`: CPU и памяти подов. Особое внимание на `averageUtulization` — это значение отражает целевой процент ресурсов, который был **реквестирован**.

{% raw %}

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  # Границы масштабирования контроллера.
  minReplicas: 1
  maxReplicas: 10
  # Если для приложения характерны кратковременные скачки потребления CPU,
  # можно отложить принятие решения о масштабировании, чтобы убедиться, что оно необходимо.
  # По умолчанию масштабирование вверх происходит немедленно.
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
  metrics:
  # Масштабирование по CPU и памяти.
  - type: Resource
    resource:
      name: cpu
      target:
        # Масштабирование, когда среднее использование CPU всех подов в scaleTargetRef превышает заданное значение.
        # Для метрики с type: Resource доступен только type: Utilization.
        type: Utilization
        # Масштабирование, если для всех подов из Deployment запрошено по 1 ядру и в среднем уже используется более 700m.
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        # Пример масштабирования, когда среднее использование памяти всех подов в scaleTargetRef превышает заданное значение.
        type: Utilization
        # Масштабирование, если для подов запрошено по 1 ГБ памяти и в среднем использовано уже более 800 МБ.
        averageUtilization: 80
```

{% endraw %}

## Масштабирование по кастомным метрикам

### Регистрация кастомных метрик в Kubernetes API

Кастомные метрики необходимо регистрировать в API `/apis/custom.metrics.k8s.io/`, эту регистрацию производит prometheus-metrics-adapter (и он же реализует API). На эти метрики можно будет ссылаться из объекта _HorizontalPodAutoscaler_. Настройка ванильного prometheus-metrics-adapter — трудоемкий процесс, мы его упростили, определив набор [Custom Resources](cr.html) с разным Scope:
* Namespaced:
  * `ServiceMetric`;
  * `IngressMetric`;
  * `PodMetric`;
  * `DeploymentMetric`;
  * `StatefulsetMetric`;
  * `NamespaceMetric`;
  * `DaemonSetMetric` (недоступен пользователям).
* Cluster:
  * `ClusterServiceMetric` (недоступен пользователям);
  * `ClusterIngressMetric` (недоступен пользователям);
  * `ClusterPodMetric` (недоступен пользователям);
  * `ClusterDeploymentMetric` (недоступен пользователям);
  * `ClusterStatefulsetMetric` (недоступен пользователям);
  * `ClusterDaemonSetMetric` (недоступен пользователям).

С помощью cluster-wide-ресурса можно задать глобальное определение метрики, а с помощью _Namespace_ можно переопределить её локально. [Формат](cr.html) для всех custom resource — одинаковый.

### Применяем кастомные метрики в HPA

После регистрации кастомной метрики на нее можно ссылаться. С точки зрения HPA, кастомные метрики бывают двух видов — `Pods` и `Object`.

`Object` — отсылает к объекту в кластере, который имеет в Prometheus метрики с соответствующими лейблами (`namespace=XXX,ingress=YYY`). Эти лейблы будут подставляться вместо `<<.LabelMatchers>>` в вашем кастомном запросе.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1beta1
kind: IngressMetric
metadata:
  name: mymetric
  namespace: mynamespace
spec:
  query: sum(rate(ingress_nginx_detail_requests_total{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>) OR on() vector(0)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  # Метрики, используемые для масштабирования.
  # Пример использования кастомных метрик.
  metrics:
  - type: Object
    object:
      # Объект, который обладает метриками в Prometheus.
      describedObject:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        name: myingress
      metric:
        # Метрика, зарегистрированная с помощью custom resource IngressMetric или ClusterIngressMetric.
        # Можно использовать rps_1m, rps_5m или rps_15m которые поставляются с модулем prometheus-metrics-adapter.
        name: mymetric
      target:
        # Для метрик типа Object можно использовать `Value` или `AverageValue`.
        type: AverageValue
        # Масштабирование происходит, если среднее значение кастомной метрики для всех подов в Deployment сильно отличается от 10.
        averageValue: 10
```

{% endraw %}

`Pods` — из ресурса, которым управляет HPA, будут выбраны все поды и для каждого пода будут собраны метрики с соответствующими лейблами (`namespace=XXX`, `pod=YYY-sadiq`, `namespace=XXX`, `pod=YYY-e3adf`, и т. д.). Из этих показателей HPA рассчитает среднее значение и использует для [масштабирования](#примеры-с-использованием-кастомных-метрик-типа-pods).

#### Пример использования кастомных метрик с размером очереди RabbitMQ

В представленном примере рассматривается очередь `send_forum_message` в RabbitMQ, для которого зарегистрирован сервис `rmq`. Если количество сообщений в этой очереди превышает 42, выполняется масштабирование.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1beta1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myconsumer
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Object
    object:
      describedObject:
        apiVersion: v1
        kind: Service
        name: rmq
      metric:
        name: rmq-queue-forum-messages
      target:
        type: Value
        value: 42
```

{% endraw %}

#### Пример использования нестабильной кастомной метрики

Улучшение предыдущего примера.

В представленном примере рассматривается очередь `send_forum_message` в RabbitMQ, для которого зарегистрирован сервис `rmq`. Если количество сообщений в этой очереди превышает 42, выполняется масштабирование. Мы не хотим реагировать на краткосрочные всплески, поэтому используется MQL-функцию `avg_over_time()`, чтобы усреднить метрику.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1beta1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (avg_over_time(rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}[5m])) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myconsumer
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Object
    object:
      describedObject:
        apiVersion: v1
        kind: Service
        name: rmq
      metric:
        name: rmq-queue-forum-messages
      target:
        type: Value
        value: 42
```

{% endraw %}

#### Примеры с использованием кастомных метрик типа `Pods`
Пример масштабирования воркеров по процентному количеству активных php-fpm-воркеров.
В представленом примере среднее количество php-fpm-воркеров в _Deployment_ `mybackend` не больше 5.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: mybackend
  minReplicas: 1
  maxReplicas: 5
  metrics:
  # Указание HPA обойти все поды Deployment'а и собрать с них метрики.
  - type: Pods
    # Указывать describedObject в отличие от type: Object не надо.
    pods:
      metric:
        # Кастомная метрика, зарегистрированная с помощью custom resource PodMetric.
        name: php-fpm-active-workers
      target:
        # Для метрик с type: Pods можно использовать только AverageValue.
        type: AverageValue
        # Масштабирование, если среднее значение метрики у всех подов Deployment'а больше 5.
        averageValue: 5
```

{% endraw %}

Масштабируется Deployment по процентному количеству активных php-fpm-воркеров.

{% raw %}

```yaml
---
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-worker
spec:
  # Процент активных php-fpm-воркеров. Функция round() для того, чтобы не смущаться от миллипроцентов в HPA.
  query: round(sum by(<<.GroupBy>>) (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) / sum by(<<.GroupBy>>) (phpfpm_processes_total{<<.LabelMatchers>>}) * 100)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: {{ .Chart.Name }}-hpa
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1beta1
    kind: Deployment
    name: {{ .Chart.Name }}
  minReplicas: 4
  maxReplicas: 8
  metrics:
  - type: Pods
    pods:
      metric:
        name: php-fpm-active-worker
      target:
        type: AverageValue
        # Масштабирование, если в среднем по Deployment 80% воркеров заняты.
        averageValue: 80
```

{% endraw %}

### Регистрация внешних метрик в Kubernetes API

Модуль `prometheus-metrics-adapter` поддерживает механизм `externalRules`, с помощью которого можно определять кастомные PromQL-запросы и регистрировать их как метрики.

В примерах инсталляций добавлено универсальное правило, которое позволяет создавать собственные метрики без настроек в `prometheus-metrics-adapter`, — «любая метрика в Prometheus с именем `kube_adapter_metric_<name>` будет зарегистрирована в API под именем `<name>`». После чего, остается написать экспортер (exporter), который будет экспортировать подобную метрику, или создать правило [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) в Prometheus, которое будет агрегировать вашу метрику на основе других метрик.

Пример _CustomPrometheusRules_:

В примере представлены пользовательские правила Prometheus для метрики `mymetric`.

В примере представлены пользовательские правила Prometheus для метрики `mymetric`.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # Рекомендованный шаблон для названия ваших CustomPrometheusRules.
  name: prometheus-metrics-adapter-mymetric
spec:
  groups:
  # Рекомендованный шаблон.
  - name: prometheus-metrics-adapter.mymetric
    rules:
    # Название вашей новой метрики.
    # Важно! Префикс 'kube_adapter_metric_' обязателен.
    - record: kube_adapter_metric_mymetric
      # Запрос, результаты которого попадут в итоговую метрику, нет смысла тащить в нее лишние лейблы.
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress)
```

{% endraw %}

### Применение внешних метрик в HPA

После регистрации внешней метрики на нее можно сослаться.

{% raw %}

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # Указывается контроллер, который нужно масштабировать (ссылка на deployment или statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:
  # Используем внешние метрики для масштабирования.
  - type: External
    external:
      metric:
        # Метрика, которую мы зарегистрировали с помощью создания метрики в Prometheus kube_adapter_metric_mymetric, но без префикса 'kube_adapter_metric_'.
        name: mymetric
        selector:
          # Для внешних метрик можно и нужно уточнять запрос с помощью лейблов.
          matchLabels:
            namespace: mynamespace
            ingress: myingress
      target:
        # Для метрик типа External можно использовать только `type: Value`.
        type: Value
        # Масштабирование, если значение нашей метрики больше 10.
        value: 10
```

{% endraw %}

### Пример с размером очереди в Amazon SQS

Чтобы установить экспортер для интеграции с SQS:
1. Cоздайте отдельный "служебный" репозиторий Git (или, к примеру, можно использовать "инфраструктурный" репозиторий).
1. Pазместите в нем инсталляцию экспортера и сценарий для создания требуемого _CustomPrometheusRules_.

Готово, вы объединили кластер. Если необходимо настроить автомасштабирование только для одного приложения (в одном пространстве имен), лучше ставить экспортер вместе с этим приложением и воспользоваться `NamespaceMetrics`.

Ниже приведен пример экспортера (например, [sqs-exporter](https://github.com/ashiddo11/sqs-exporter)) для получения метрик из Amazon SQS, если:
* в Amazon SQS работает очередь `send_forum_message`;
* выполняется масштабирование при количестве сообщений в этой очереди больше 42.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1beta1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (avg_over_time(rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}[5m])) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The targets of scaling (link to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myconsumer
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Object
    object:
      describedObject:
        apiVersion: v1
        kind: Service
        name: rmq
      metric:
        name: rmq-queue-forum-messages
      target:
        type: Value
        value: 42
```

{% endraw %}

#### Examples of using custom metrics of the `Pods` type

Suppose we want the average number of php-fpm workers in the `mybackend` Deployment to be no more than 5.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The targets of scaling (link to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: mybackend
  minReplicas: 1
  maxReplicas: 5
  metrics:
  # HPA must go through all the Pods in the Deployment and collect metrics from them.
  - type: Pods
    # You do not need to specify descripedObject (in contrast to type: Object).
    pods:
      metric:
        # Custom metric, registered using the PodMetric CR.
        name: php-fpm-active-workers
      target:
        # For type: Pods metrics, the AverageValue can only be used.
        type: AverageValue
        # Scale up if the average metric value for all the Pods of the myworker Deployment is greater than 5.
        averageValue: 5
```

{% endraw %}

The Deployment is scaled based on the percentage of active php-fpm workers.

{% raw %}

```yaml
---
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-worker
spec:
  # Percentage of active php-fpm workers. The round() function rounds the percentage.
  query: round(sum by(<<.GroupBy>>) (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) / sum by(<<.GroupBy>>) (phpfpm_processes_total{<<.LabelMatchers>>}) * 100)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: {{ .Chart.Name }}-hpa
spec:
  # The targets of scaling (link to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1beta1
    kind: Deployment
    name: {{ .Chart.Name }}
  minReplicas: 4
  maxReplicas: 8
  metrics:
  - type: Pods
    pods:
      metric:
        name: php-fpm-active-worker
      target:
        type: AverageValue
        # Scale up if, on average, 80% of workers in the deployment are running at full capacity.
        averageValue: 80
```

{% endraw %}

### Registering external metrics with the Kubernetes API

The `prometheus-metrics-adapter` module supports the `externalRules` mechanism. Using it, you can create custom PromQL requests and register them as metrics.

In our installations, we have implemented a universal rule that allows you to create your metrics without using `prometheus-metrics-adapter` — "any Prometheus metric called `kube_adapter_metric_<name>` will be registered in the API under the `<name>`". In other words, all you need is to either write an exporter (to export the metric) or create a [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) in Prometheus that will aggregate your metric based on other metrics.

An example of `CustomPrometheusRules`:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # The recommended template for naming your CustomPrometheusRules.
  name: prometheus-metrics-adapter-mymetric
spec:
  groups:
    # Recommended template for the name key.
  - name: prometheus-metrics-adapter.mymetric
    rules:
    # The name of the new metric. Pay attention! The 'kube_adapter_metric_' prefix is required.
    - record: kube_adapter_metric_mymetric
      # The results of this request will be passed to the final metric; there is no reason to include excess labels into it.
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress)
```

{% endraw %}

### Using external metrics with HPA

You can refer to a metric after it is registered.

{% raw %}

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The targets of scaling (link to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  # Use external metrics for scaling.
  metrics:
  - type: External
    external:
      metric:
        # The metric that we registered by creating a metric in Prometheus's kube_adapter_metric_mymetric but without 'kube_adapter_metric_' prefix.
        name: mymetric
        selector:
          # For external metrics, you can and should specify matching labels.
          matchLabels:
            namespace: mynamespace
            ingress: myingress
      target:
        # Only `type: Value` can be used for metrics of the External type.
        type: Value
        # Scale up if the value of our metric is greater than 10.
        value: 10
```

{% endraw %}

### Example of scaling based on the Amazon SQS queue size

> Note that an exporter is required to integrate with SQS. For this, create a separate "service" git repository (or you can use an "infrastructure" repository) and put the installation of this exporter as well as the script to create the necessary `CustomPrometheusRules` into this repository. If you need to configure autoscaling for a single application (especially if it runs in a single namespace), we recommend putting the exporter together with the application and using `NamespaceMetrics`.

Suppose there is a `send_forum_message` queue in Amazon SQS. Then, suppose, we want to scale up the cluster if there are more than 42 messages in the queue. Also, you will need an exporter to collect Amazon SQS metrics (say, [sqs-exporter](https://github.com/ashiddo11/sqs-exporter)).

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # The recommended name — prometheus-metrics-adapter-<metric name>.
  name: prometheus-metrics-adapter-sqs-messages-visible
  # Pay attention!
  namespace: d8-monitoring
  labels:
    # Pay attention!
    prometheus: main
    # Pay attention!
    component: rules
spec:
  groups:
  - name: prometheus-metrics-adapter.sqs_messages_visible # the recommended template
    rules:
    - record: kube_adapter_metric_sqs_messages_visible # Pay attention! The 'kube_adapter_metric_' prefix is required.
      expr: sum (sqs_messages_visible) by (queue)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The targets of scaling (link to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myconsumer
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: External
    external:
      metric:
        # Must match CustomPrometheusRules record name without 'kube_adapter_metric_' prefix.
        name: sqs_messages_visible
        selector:
          matchLabels:
            queue: send_forum_messages
      target:
        type: Value
        value: 42
```

{% endraw %}

## Debugging

### How do I get a list of custom metrics?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

### How do I get the value of a metric associated with an object?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/ingresses/*/rps_1m
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/ingresses/*/mymetric
```

### How do I get the value of a metric created via `NamespaceMetric`?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

### How do I get external metrics?

```shell
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1/namespaces/d8-ingress-nginx/d8_ingress_nginx_ds_cpu_utilization
```
