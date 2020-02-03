Модуль prometheus-metrics-adapter
=================================

## Назначение

**TLDR;** — модуль позволяет работать [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)- и [VPA](https://github.com/deckhouse/deckhouse/blob/master/modules/302-vertical-pod-autoscaler/README.md)- автоскейлерам по «любым» метрикам.

Данный модуль устанавливает в кластер [имплементацию](https://github.com/DirectXMan12/k8s-prometheus-adapter) Kubernetes [resource metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md), [custom metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/custom-metrics-api.md) и [external metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/external-metrics-api.md) для получения метрик из Prometheus.

Это позволяет:
- kubectl top брать метрики из Prometheus, через адаптер, а не из heapster;
- использовать [autoscaling/v2beta2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#metricspec-v2beta2-autoscaling) для скейлинга приложений (HPA);
- получать информацию из prometheus средствами API kubernetes для других модулей (Vertical Pod Autoscaler, ...).

Данный модуль позволяет производить скейлинг по таким параметрам:
* cpu (pod'а),
* memory (pod'а),
* rps (ingress'а) - за 1,5,15 минут (`rps_Nm`),
* cpu (pod'а) - за 1,5,15 минут (`cpu_Nm`) - среднее потребления CPU за N минут,
* memory (pod'a) - за 1,5,15 минут (`memory_Nm`) - среднее потребление Memory за N минут,
* любые Prometheus-метрики и любые запросы на их основе.

##  Конфигурация

В общем случае не требует конфигурации.

По умолчанию — **включен** в кластерах начиная с версии 1.9, если включен модуль `prometheus`.

### Параметры

* `highAvailability` — ручное управление [режимом отказоустойчивости](/FEATURES.md#отказоустойчивость).

## Как работает

Данный модуль регистрирует k8s-prometheus-adapter в качестве external API сервиса, который расширяет возможности Kubernetes API. Когда какому-то из компонентов Kubernetes (VPA, HPA) требуется информация об используемых ресурсах, он делает запрос в Kubernetes API, а тот, в свою очередь, проксирует запрос в адаптер. Адаптер на основе своего [конфигурационного файла](templates/config-map.yaml) выясняет, как посчитать метрику и отправляет запрос в Prometheus.

## Как настраивать HPA?

В данной инструкции рассматриваем только HPA с [apiVersion: autoscaling/v2beta2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#horizontalpodautoscalerspec-v2beta2-autoscaling), чья поддержка появилась начиная с Kuberntetes v1.12.

В общем виде для настройки HPA требуется:
* определить, что скейлим (`.spec.scaleTargetRef`),
* определить диапазон скейлинга (`.spec.minReplicas`, `.scale.maxReplicas`),
* зарегистрировать в API Kubernetes и определить метрики, на основе которых будем скейлиться (`.spec.metrics`).

Метрики с точки зрения HPA бывают трёх видов:
* [классические](#классический-скейлинг-по-потреблению-ресурсов) — с типом (`.spec.metrics[].type`) "Resource", используются для простейшего скейлинга по потреблению процессора и памяти,
* [кастомные](#скейлинг-по-кастомным-метрикам) — с типами (`.spec.metrics[].type`) "Pods" или "Object".
* [внешние](#скейлинг-по-внешним-метрикам) — с типом (`.spec.metrics[].type`) "External".

### Какой тип скейлинга мне подойдёт?

1. С [классическим](#классический-скейлинг-по-потреблению-ресурсов) всё понятно.
1. Если у вас одно приложение, источник метрик находится внутри Namespace и он связан с одним из объектов, то используйте [кастомные](#скейлинг-по-кастомным-метрикам) Namespace-scoped метрики.
1. Если у вас много приложений используют одинаковую метрику, источник которой находится в Namespace приложения и которая связана с одним из объектов — используйте [кастомные](#скейлинг-по-кастомным-метрикам) Cluster-wide метрики. Подобные метрики предусмотрены на случай необходимости выделения общих инфраструктурных компонентов в отдельный деплой ("infra").
1. Если источник метрики не привязан к Namespace приложения — используйте [внешние](#скейлинг-по-внешним-метрикам) метрики. Например, метрики cloud-провайдера или внешней SaaS-ки.

**Важно!** Категорически рекомендуется пользоваться или 1. [классическими](#классический-скейлинг-по-потреблению-ресурсов) метриками, или 2. [кастомными](#скейлинг-по-кастомным-метрикам) метриками, определяемыми в Namespace, так как в этом случае вы можете определить всю конфигурацию приложения, включая логику его автомасштабирования, в репозитарии самого приложения. Варианты 3 и 4 стоит рассматривать только если у вас большая коллекция идентичных микросервисов.

### Классический скейлинг по потреблению ресурсов

Пример HPA для скейлинга по базовым метрикам из `metrics.k8s.io`: CPU и Memory Pod'ов. Особое внимание на `averageUtulization` — это значение отражает целевой процент ресурсов, который был **реквестирован**.

```yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  # что скейлим?
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  # "от" и "до"
  minReplicas: 1
  maxReplicas: 10
  metrics:
  # скейлимся по CPU и Memory
  - type: Resource
    resource:
      name: cpu
      target:
        # скейлимся, когда среднее использование CPU всех подов в scaleTargetRef превышает заданное значение
        type: Utilization      # для метрики с type: Resource доступен только type: Utilization
        averageUtilization: 70 # если все поды из деплоймента реквестировали по 1 ядру и в среднем съели более 700m, — скейлимся
  - type: Resource
    resource:
      name: memory
      target:
        # скейлимся, когда среднее использование Memory всех подов в scaleTargetRef превышает заданное значение
        type: Utilization
        averageUtilization: 80 # если поды реквестировали по 1GB памяти и в среднем съели более 800MB, — скейлимся
```

### Скейлинг по кастомным метрикам

#### Регистрируем кастомные метрики в Kubernetes API

Кастомные метрики необходимо регистрировать в API `/apis/custom.metrics.k8s.io/`, в нашем случае эту регистрацию производит prometheus-metrics-adapter (и он же реализует API). Потом на эти метрики можно будет ссылаться из объекта `HorizontalPodAutoscaler`. Настройка ванильного prometheus-metrics-adapter-а — это достаточно трудоёмкий процесс и мы его несколько упростили, определив набор **CRD** с разным Scope:
* Namespaced:
    * `ServiceMetric`
    * `IngressMetric`
    * `PodMetric`
    * `DeploymentMetric`
    * `StatefulsetMetric`
    * `NamespaceMetric`
    * `DaemonsetMetric` (не доступен пользователям)
* Cluster:
    * `ClusterServiceMetric` (не доступен пользователям)
    * `ClusterIngressMetric` (не доступен пользователям)
    * `ClusterPodMetric` (не доступен пользователям)
    * `ClusterDeploymentMetric` (не доступен пользователям)
    * `ClusterStatefulsetMetric` (не доступен пользователям)
    * `ClusterDaemonsetMetric` (не доступен пользователям)

С помощью Cluster-ресурса можно определить метрику глобально, а с помощью Namespaced-ресурса можно её локально переопределять. Формат у всех CRD одинаковый:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressMetric
metadata:
  name: mymetric
  namespace: mynamespace
spec:
  query: sum(ingress_nginx_detail_requests_total{<<.LabelMatchers>>}) by (<<.GroupBy>>)
```
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterIngressMetric
metadata:
  name: mymetric
spec:
  query: sum(ingress_nginx_detail_sent_bytes_sum{<<.LabelMatchers>>}) by (<<.GroupBy>>)
```

Где:
* `.metadata.name` — имя метрики, используется в HPA.
* `.spec.query` — кастомный PromQL-запрос, который возвращает однозначное значение для вашего набора лейблов (используйте группировку операторами `sum() by()`, `max() by()` и пр.). В запросе необходимо **обязательно использовать** ключевики:
    * `<<.LabelMatchers>>` — заменится на набор лейблов `{namespace="mynamespace",ingress="myingress"}`. Можно добавить свои лейблы через запятую как в [примере ниже](#пример-с-размером-очереди-rabbitmq).
    * `<<.GroupBy>>` — заменится на перечисление лейблов `namespace,ingress` для группировки (`max() by(...)`, `sum() by (...)` и пр.).

#### Применяем кастомные метрики в HPA

После регистрации кастомной метрики можно на неё сослаться. С точки зрения HPA, кастомные метрики бывают двух видов — `Pods` и `Object`. С `Object` всё просто — это отсылка к объекту в кластере, который имеет в прометее метрики с соответствующими лейблами (`namespace=XXX,ingress=YYY`). Эти лейблы будут подставляться вместо `<<.LabelMatchers>>` в вашем кастомном запросе.

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Что скейлим (ссылка на deployment или statefulset).
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:              # Какие метрики использовать для скейла? Мы используем кастомные типа Object.
  - type: Object
    object:
      describedObject:  # Некий объект, который обладает метриками в прометее.
        apiVersion: extensions/v1beta1
        kind: Ingress
        name: myingress
      metric:
        name: mymetric  # Метрика, которую мы зарегистрировали с помощью CRD IngressMetric или ClusterIngressMetric.
      target:
        type: Value     # Для метрик типа Object можно использовать только `type: Value`.
        value: 10       # Если значение нашей кастомной метрики больше 10, то надо скейлиться!
```

C `Pods` сложнее — из ресурса, который скейлит HPA, будут извлечены все pod-ы и по каждому будет собраны метрики с соответствующими лейблами (`namespace=XXX,pod=YYY-sadiq`,`namespace=XXX,pod=YYY-e3adf`,...). Из этих метрик HPA посчитает среднее и использует для скейлинга. См. [пример ниже](#пример-с-использованием метрик-типа-pods).

### Пример использования кастомных метрик с размером очереди RabbitMQ

Имеем очередь "send_forum_message" в реббите, для которого зарегистрирован сервис "rmq". Если сообщений в очереди больше 42 — скейлимся.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
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

### Пример с использованием кастомных метрик типа `Pods`

Хотим, чтобы среднее количество php-fpm-воркеров в деплойменте "mybackend" было не больше 5.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Что скейлим.
    apiVersion: apps/v1
    kind: Deployment
    name: mybackend
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Pods # Просим HPA самому обойти все поды нашего деплоймента и собрать с них метрики.
    pods:      # Указывать describedObject в отличие от type: Object не надо.
      metric:
        name: php-fpm-active-workers # Кастомная метрика, которую мы зарегистрировали с помощью CRD PodMetric.
      target:
        type: AverageValue # Для метрик с type: Pods можно использовать только AverageValue.
        averageValue: 5   # Если среднее значение метрики у всех подов деплоя myworker больше 5, то скейлимся.
```

### Скейлинг по внешним метрикам

**Важно!** Настраивать скейлинг по внешним метрикам (добавлять PrometheusRule в namespace d8-monitoring) корректно ТОЛЬКО из некоторого инфраструктурного репозитария, но никак не из git-репозитариев прилоежний. Всячески старайтесь вместо этого использовать, например, NamespaceMetric.

#### Регистрируем внешние метрики в Kubernetes API

Prometheus-metrics-adapter поддерживает механизм `externalRules`, с помощью которого можно определять кастомные PromQL-запросы и регистрировать их как метрики. В наших инсталляциях мы добавили универсальное правило, которое позволяет создавать свои метрики без внесения настроек в prometheus-metrics-adapter — "любая метрика в Prometheus с именем `kube_adapter_metric_<name>` будет зарегистрирована в API под именем `<name>`". То есть, остаётся либо написать exporter, который будет экспортировать подобную метрику, либо создать [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) в прометее, которое будет аггрегировать вашу метрику на основе других метрик.

Пример PrometheusRule:

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

#### Применяем внешние метрики в HPA

После регистрации внешней метрики можно на неё сослаться.

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Что скейлим (ссылка на deployment или statefulset).
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:              # Какие метрики использовать для скейла? Мы используем внешние метрики.
  - type: External
    metric:
      name: mymetric  # Метрика, которую мы зарегистрировали с помощью создания метрики в прометее kube_adapter_metric_mymetric.
      selector:
        matchLabels:  # Для внешних метрик можно и нужно уточнять запрос с помощью лейблов.
          namespace: mynamespace
          ingress: myingress
    target:
      type: Value     # Для метрик типа External можно использовать только `type: Value`.
      value: 10       # Если значение нашей метрики больше 10, то надо скейлиться!
```

#### Пример с размером очереди в Amazon SQS

**Важно!** Для интеграции с SQS вам понадобится установка экспортера – нужно завести отдельный "служебный" git-репозитарий для этих целей (или, лучше, использовать "инфраструктурный" репозитарий) и разместить в нем установку этого экспортера и создание необходимого PrometheusRule, таким образом интегрировав кластер. Если же вам нужно настроить автомасштабирование только для одного приложения (особенно живущего в одном неймспейсе), лучше ставить экспортер вместе с этим приложением и воспользоваться NamespaceMetrics.

В Amazon SQS работает очередь "send_forum_message". Если сообщений в очереди больше 42 — скейлимся. Для получения метрик из Amazon SQS понадобится экспортер, для примера — [sqs-exporter](https://github.com/ashiddo11/sqs-exporter).

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: prometheus-metrics-adapter-sqs-messages-visible # Рекомендованное название — prometheus-metrics-adapter-<metric name>
  namespace: d8-monitoring # Важно!
  labels:
    prometheus: main # Важно!
    component: rules # Важно!
spec:
  groups:
  - name: prometheus-metrics-adapter.sqs_messages_visible # Рекомендованный шаблон.
    rules:
    - record: kube_adapter_metric_sqs_messages_visible
      expr: sum (sqs_messages_visible) by (queue)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
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
        name: sqs_messages_visible
        selector:
          matchLabels:
            queue: send_forum_messages
      target:
        type: Value
        value: 42
```

## Отладка

Получить список кастомных метрик:

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

Получить значение метрики, привязанной к объекту:

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
```

Получить значение метрики, созданной через `NamespaceMetric`:

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

Получить external-метрики:

```shell
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1"
```
