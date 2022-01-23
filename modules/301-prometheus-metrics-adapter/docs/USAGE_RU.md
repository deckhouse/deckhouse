---
title: "Модуль prometheus-metrics-adapter: примеры конфигурации"
search: autoscaler, HorizontalPodAutoscaler 
---

Далее рассматривается только HPA с [apiVersion: autoscaling/v2beta2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#horizontalpodautoscalerspec-v2beta2-autoscaling), чья поддержка появилась начиная с Kubernetes v1.12.

В общем виде, для настройки HPA требуется:
* определить, что масштабируем (`.spec.scaleTargetRef`);
* определить диапазон масштабирования (`.spec.minReplicas`, `.scale.maxReplicas`);
* зарегистрировать в API Kubernetes и определить метрики, на основе которых будем масштабировать (`.spec.metrics`).

Метрики с точки зрения HPA бывают трёх видов:
* [классические](#классическое-масштабирование-по-потреблению-ресурсов) — с типом (`.spec.metrics[].type`) "Resource", используются для простейшего масштабирования по потреблению процессора и памяти;
* [кастомные](#масштабирование-по-кастомным-метрикам) — с типами (`.spec.metrics[].type`) "Pods" или "Object";
* [внешние](#применяем-внешние-метрики-в-hpa) — с типом (`.spec.metrics[].type`) "External".

**Важно!** [По умолчанию](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#default-behavior) HPA использует разные подходы при масштабировании в ту или иную сторону:
* Если метрики [говорят](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) о том, что надо масштабировать **вверх**, то это происходит немедленно (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 0). Единственное ограничение — скорость прироста: за 15 секунд Pod'ы могут максимум либо удвоиться, либо, если Pod'ов меньше 4-х, то прибавляется 4шт.
* Если метрики [говорят](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) о том, что надо масштабировать **вниз**, то это происходит плавно. В течение пяти минут (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 300) собираются предложения о новом количестве реплик, в результате чего выбирается самое большое значение. Ограничений на количество "уволенных" Pod'ов за раз нет.

Если есть проблемы с флаппингом метрик и наблюдается взрывной рост ненужных реплик приложения, то есть разные подходы:
* Обернуть метрику агрегирующей функцией (например, `avg_over_time()`), если метрика определена PromQL-запросом. [Пример...](#пример-использования-нестабильной-кастомной-метрики)
* Увеличить `spec.behavior.scaleUp.stabilizationWindowSeconds` в ресурсе `HorizontalPodAutoscaler`. В этом случае, в течение обозначенного периода будут собираться предложения об увеличении количества реплик, в результате чего будет выбрано самое скромное предложение. Иными словами, это решение тождественно применению аггрегирующей функции `min_over_time(<stabilizationWindowSeconds>)`, но только в случае если метрика растёт и требуется масштабирование **вверх**. Для масштабирования **вниз**, как правило, достаточно стандартных настроек. [Пример...](#классическое-масштабирование-по-потреблению-ресурсов)
* Сильнее ограничить скорость прироста новых реплик с помощью `spec.behavior.scaleUp.policies`.

## Какой тип масштабирования мне подойдёт?

1. С [классическим](#классическое-масштабирование-по-потреблению-ресурсов) всё понятно.
1. Если у вас одно приложение, источник метрик находится внутри Namespace и он связан с одним из объектов, то используйте [кастомные](#масштабирование-по-кастомным-метрикам) Namespace-scoped-метрики.
1. Если у вас много приложений используют одинаковую метрику, источник которой находится в Namespace приложения и которая связана с одним из объектов — используйте [кастомные](#масштабирование-по-кастомным-метрикам) Cluster-wide-метрики. Подобные метрики предусмотрены на случай необходимости выделения общих инфраструктурных компонентов в отдельный деплой ("infra").
1. Если источник метрики не привязан к Namespace приложения — используйте [внешние](#применяем-внешние-метрики-в-hpa) метрики. Например, метрики cloud-провайдера или внешнего SaaS-сервиса.

**Важно!** Настоятельно рекомендуется пользоваться или Вариантом 1. ([классическими](#классическое-масштабирование-по-потреблению-ресурсов) метриками), или Вариантом 2. ([кастомными](#масштабирование-по-кастомным-метрикам) метриками, определяемыми в Namespace), так как в этом случае вы можете определить всю конфигурацию приложения, включая логику его автомасштабирования, в репозитарии самого приложения. Варианты 3 и 4 стоит рассматривать только если у вас большая коллекция идентичных микросервисов.

## Классическое масштабирование по потреблению ресурсов

Пример HPA для масштабирования по базовым метрикам из `metrics.k8s.io`: CPU и Memory Pod'ов. Особое внимание на `averageUtulization` — это значение отражает целевой процент ресурсов, который был **реквестирован**.

{% raw %}
```yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  # что масштабируем
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  # "от" и "до"
  minReplicas: 1
  maxReplicas: 10
  behavior:                           # если приложению характерны кратковременные скачки потребления CPU,
    scaleUp:                          # можно отложить принятие решения о масштабировании, чтобы убедиться что он необходим
      stabilizationWindowSeconds: 300 # (по умолчанию, масштабирование вверх происходит немедленно)
  metrics:
  # масштабируем по CPU и Memory
  - type: Resource
    resource:
      name: cpu
      target:
        # масштабируем, когда среднее использование CPU всех Pod'ов в scaleTargetRef превышает заданное значение
        type: Utilization      # для метрики с type: Resource доступен только type: Utilization
        averageUtilization: 70 # если все Pod'ы из Deployment реквестировали по 1 ядру и в среднем съели более 700m, — масштабируем
  - type: Resource
    resource:
      name: memory
      target:
        # масштабируем, когда среднее использование Memory всех Pod'ов в scaleTargetRef превышает заданное значение
        type: Utilization
        averageUtilization: 80 # если Pod'ы реквестировали по 1GB памяти и в среднем съели более 800MB, — масштабируем
```
{% endraw %}

## Масштабирование по кастомным метрикам

### Регистрируем кастомные метрики в Kubernetes API

Кастомные метрики необходимо регистрировать в API `/apis/custom.metrics.k8s.io/`, в нашем случае эту регистрацию производит `prometheus-metrics-adapter` (и он же реализует API). Потом на эти метрики можно будет ссылаться из объекта `HorizontalPodAutoscaler`. Настройка ванильного `prometheus-metrics-adapter` достаточно трудоёмкий процесс, но мы его несколько упростили, определив набор [Custom Resources](cr.html) с разным Scope:
* Namespaced:
    * `ServiceMetric`
    * `IngressMetric`
    * `PodMetric`
    * `DeploymentMetric`
    * `StatefulsetMetric`
    * `NamespaceMetric`
    * `DaemonSetMetric` (недоступен пользователям)
* Cluster:
    * `ClusterServiceMetric` (недоступен пользователям)
    * `ClusterIngressMetric` (недоступен пользователям)
    * `ClusterPodMetric` (недоступен пользователям)
    * `ClusterDeploymentMetric` (недоступен пользователям)
    * `ClusterStatefulsetMetric` (недоступен пользователям)
    * `ClusterDaemonSetMetric` (недоступен пользователям)

С помощью Cluster-scoped-ресурса можно определить метрику глобально, а с помощью Namespaced-ресурса можно её локально переопределять. [Формат](cr.html) у всех CR — одинаковый.

### Применяем кастомные метрики в HPA

После регистрации кастомной метрики на нее можно сослаться. С точки зрения HPA, кастомные метрики бывают двух видов — `Pods` и `Object`. С `Object` всё просто — это отсылка к объекту в кластере, который имеет в Prometheus метрики с соответствующими лейблами (`namespace=XXX,ingress=YYY`). Эти лейблы будут подставляться вместо `<<.LabelMatchers>>` в вашем кастомном запросе.

{% raw %}
```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Что масштабируем (ссылка на deployment или statefulset).
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:              # Какие метрики использовать для масштабирования? Мы используем кастомные, типа Object.
  - type: Object
    object:
      describedObject:  # Некий объект, который обладает метриками в Prometheus.
        apiVersion: extensions/v1beta1
        kind: Ingress
        name: myingress
      metric:
        name: mymetric  # Метрика, которую мы зарегистрировали с помощью CR IngressMetric или ClusterIngressMetric.
      target:
        type: Value     # Для метрик типа Object можно использовать только `type: Value`.
        value: 10       # Если значение нашей кастомной метрики больше 10, то масштабируем
```
{% endraw %}

C `Pods` сложнее — из ресурса, который масштабирует HPA, будут извлечены все Pod'ы и по каждому будет собраны метрики с соответствующими лейблами (`namespace=XXX,pod=YYY-sadiq`,`namespace=XXX,pod=YYY-e3adf`,...). Из этих метрик HPA посчитает среднее и использует для масштабирования. [Пример...](#примеры-с-использованием-кастомных-метрик-типа-pods)

#### Пример использования кастомных метрик с размером очереди RabbitMQ

Имеем очередь `send_forum_message` в RabbitMQ, для которого зарегистрирован сервис `rmq`. Если сообщений в очереди больше 42 — масштабируем.

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
{% endraw %}

#### Пример использования нестабильной кастомной метрики

Улучшение предыдущего примера. 

Имеем очередь `send_forum_message` в RabbitMQ, для которого зарегистрирован сервис `rmq`. Если сообщений в очереди больше 42 — масштабируем.
При этом мы не хотим реагировать на кратковременные вспышки, для этого усредняем метрику с помощью MQL-функции `avg_over_time()`.

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
{% endraw %}

#### Примеры с использованием кастомных метрик типа `Pods`

Хотим, чтобы среднее количество php-fpm-воркеров в Deployment `mybackend` было не больше 5.

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
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Что масштабируем
    apiVersion: apps/v1
    kind: Deployment
    name: mybackend
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Pods # Просим HPA самому обойти все Pod'ы нашего Deployment'а и собрать с них метрики.
    pods:      # Указывать describedObject в отличие от type: Object не надо.
      metric:
        name: php-fpm-active-workers # Кастомная метрика, которую мы зарегистрировали с помощью CR PodMetric.
      target:
        type: AverageValue # Для метрик с type: Pods можно использовать только AverageValue.
        averageValue: 5   # Если среднее значение метрики у всех Pod'ов Deployment'а myworker больше 5, то масштабируем.
```
{% endraw %}

Масштабируем Deployment по процентному количеству active-воркеров php-fpm.

{% raw %}
```yaml
---
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-worker
spec:
  query: round(sum by(<<.GroupBy>>) (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) / sum by(<<.GroupBy>>) (phpfpm_processes_total{<<.LabelMatchers>>}) * 100) # Процент active-воркеров в php-fpm. Функция round() для того, чтобы не смущаться от милли-процентов в HPA.
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: {{ .Chart.Name }}-hpa
spec:
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
        averageValue: 80 # Если в среднем по деплойменту 80% воркеров заняты, масштабируем
```
{% endraw %}

### Регистрируем внешние метрики в Kubernetes API

Модуль `prometheus-metrics-adapter` поддерживает механизм `externalRules`, с помощью которого можно определять кастомные PromQL-запросы и регистрировать их как метрики. 

В наших инсталляциях мы добавили универсальное правило, которое позволяет создавать свои метрики без внесения настроек в `prometheus-metrics-adapter` — "любая метрика в Prometheus с именем `kube_adapter_metric_<name>` будет зарегистрирована в API под именем `<name>`". То есть, остаётся либо написать exporter, который будет экспортировать подобную метрику, либо создать [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) в Prometheus, которое будет агрегировать вашу метрику на основе других метрик.

Пример `CustomPrometheusRules`:

{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: prometheus-metrics-adapter-mymetric # Рекомендованный шаблон для названия вашего CustomPrometheusRules.
spec:
  groups:
  - name: prometheus-metrics-adapter.mymetric # Рекомендованный шаблон.
    rules:
    - record: kube_adapter_metric_mymetric # Как ваша новая метрика будет называться. Важно! Префикс 'kube_adapter_metric_' обязателен
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress) # Запрос, результаты которого попадут в итоговую метрику, нет смысла тащить в неё лишние лейблы.
```
{% endraw %}

### Применяем внешние метрики в HPA

После регистрации внешней метрики, на нее можно сослаться.

{% raw %}
```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Что масштабируем (ссылка на deployment или statefulset).
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:              # Какие метрики использовать для масштабирования? Мы используем внешние метрики.
  - type: External
    external:
      metric:
        name: mymetric  # Метрика, которую мы зарегистрировали с помощью создания метрики в Prometheus kube_adapter_metric_mymetric, но без префикса 'kube_adapter_metric_'
        selector:
          matchLabels:  # Для внешних метрик можно и нужно уточнять запрос с помощью лейблов.
            namespace: mynamespace
            ingress: myingress
      target:
        type: Value     # Для метрик типа External можно использовать только `type: Value`.
        value: 10       # Если значение нашей метрики больше 10, то масштабируем
```
{% endraw %}

### Пример с размером очереди в Amazon SQS

> Для интеграции с SQS вам понадобится установка экспортера. Для этого нужно завести отдельный "служебный" git-репозитарий (или, например, использовать "инфраструктурный" репозитарий) и разместить в нем установку этого экспортера, а также создание необходимого `CustomPrometheusRules`, таким образом интегрировав кластер. Если же вам нужно настроить автомасштабирование только для одного приложения (особенно живущего в одном неймспейсе), лучше ставить экспортер вместе с этим приложением и воспользоваться `NamespaceMetrics`.

В следующем примере подразумевается, что в Amazon SQS работает очередь `send_forum_message`. Если сообщений в очереди больше 42 — масштабируем. Для получения метрик из Amazon SQS понадобится exporter (например — [sqs-exporter](https://github.com/ashiddo11/sqs-exporter)).

{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: prometheus-metrics-adapter-sqs-messages-visible # Рекомендованное название — prometheus-metrics-adapter-<metric name>
spec:
  groups:
  - name: prometheus-metrics-adapter.sqs_messages_visible # Рекомендованный шаблон.
    rules:
    - record: kube_adapter_metric_sqs_messages_visible # Важно! Префикс 'kube_adapter_metric_' обязателен
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
        name: sqs_messages_visible # Должен совпадать с CustomPrometheusRules record без префикса 'kube_adapter_metric_'
        selector:
          matchLabels:
            queue: send_forum_messages
      target:
        type: Value
        value: 42
```
{% endraw %}

## Способы отладки

### Как получить список кастомных метрик?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

### Как получить значение метрики, привязанной к объекту?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
```

### Как получить значение метрики, созданной через `NamespaceMetric`?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

### Как получить external-метрики?

```shell
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1"
```
