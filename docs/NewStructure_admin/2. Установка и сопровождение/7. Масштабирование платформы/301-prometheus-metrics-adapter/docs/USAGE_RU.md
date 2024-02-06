---
title: "Модуль prometheus-metrics-adapter: примеры конфигурации"
search: autoscaler, HorizontalPodAutoscaler
---

Далее рассматривается только HPA с [apiVersion: autoscaling/v2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#objectmetricsource-v2-autoscaling), чья поддержка появилась начиная с Kubernetes v1.12.

В общем виде для настройки HPA требуется:
* определить, что масштабируем (`.spec.scaleTargetRef`);
* определить диапазон масштабирования (`.spec.minReplicas`, `.scale.maxReplicas`);
* зарегистрировать в API Kubernetes и определить метрики, на основе которых будем масштабировать (`.spec.metrics`).

Метрики с точки зрения HPA бывают трех видов:
* [классические](#классическое-масштабирование-по-потреблению-ресурсов) — с типом (`.spec.metrics[].type`) «Resource», используются для простейшего масштабирования по потреблению процессора и памяти;
* [кастомные](#масштабирование-по-кастомным-метрикам) — с типами (`.spec.metrics[].type`) «Pods» или «Object»;
* [внешние](#применяем-внешние-метрики-в-hpa) — с типом (`.spec.metrics[].type`) «External».

**Важно!** [По умолчанию](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#default-behavior) HPA использует разные подходы при масштабировании в ту или иную сторону:
* Если метрики [говорят](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) о том, что надо масштабировать **вверх**, это происходит немедленно (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 0). Единственное ограничение — скорость прироста: за 15 секунд либо поды могут максимум удвоиться, либо, если подов меньше 4, добавятся 4 новых пода.
* Если метрики [говорят](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) о том, что надо масштабировать **вниз**, это происходит плавно. В течение 5 минут (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 300) собираются предложения о новом количестве реплик, в результате чего выбирается самое большое значение. Ограничений на количество «уволенных» подов за раз нет.

Если есть проблемы с флаппингом метрик и наблюдается взрывной рост ненужных реплик приложения, имеются разные подходы:
* Обернуть метрику агрегирующей функцией (например, `avg_over_time()`), если метрика определена PromQL-запросом. [Пример...](#пример-использования-нестабильной-кастомной-метрики)
* Увеличить `spec.behavior.scaleUp.stabilizationWindowSeconds` в ресурсе `HorizontalPodAutoscaler`. В этом случае в течение обозначенного периода будут собираться предложения об увеличении количества реплик, в результате чего будет выбрано самое скромное предложение. Иными словами, это решение тождественно применению агрегирующей функции `min_over_time(<stabilizationWindowSeconds>)`, но только в том случае, если метрика растет и требуется масштабирование **вверх**. Для масштабирования **вниз**, как правило, достаточно стандартных настроек. [Пример...](#классическое-масштабирование-по-потреблению-ресурсов)
* Сильнее ограничить скорость прироста новых реплик с помощью `spec.behavior.scaleUp.policies`.

## Какой тип масштабирования мне подойдет?

1. С [классическим](#классическое-масштабирование-по-потреблению-ресурсов) все понятно.
1. Если у вас одно приложение, источник метрик находится внутри namespace и он связан с одним из объектов, используйте [кастомные](#масштабирование-по-кастомным-метрикам) Namespace-scoped-метрики.
1. Если у вас много приложений используют одинаковую метрику, источник которой находится в namespace приложения и которая связана с одним из объектов, используйте [кастомные](#масштабирование-по-кастомным-метрикам) Cluster-wide-метрики. Подобные метрики предусмотрены на случай необходимости выделения общих инфраструктурных компонентов в отдельный деплой («infra»).
1. Если источник метрики не привязан к namespace приложения, используйте [внешние](#применяем-внешние-метрики-в-hpa) метрики. Например, метрики облачного провайдера или внешнего SaaS-сервиса.

**Важно!** Настоятельно рекомендуется пользоваться или вариантом 1 ([классическими](#классическое-масштабирование-по-потреблению-ресурсов) метриками), или вариантом 2 ([кастомными](#масштабирование-по-кастомным-метрикам) метриками, определяемыми в namespace), так как в этом случае вы можете определить всю конфигурацию приложения, включая логику его автомасштабирования, в репозитарии самого приложения. Варианты 3 и 4 стоит рассматривать, только если у вас большая коллекция идентичных микросервисов.

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

### Регистрируем кастомные метрики в Kubernetes API

Кастомные метрики необходимо регистрировать в API `/apis/custom.metrics.k8s.io/`, в нашем случае эту регистрацию производит `prometheus-metrics-adapter` (и он же реализует API). Потом на эти метрики можно будет ссылаться из объекта `HorizontalPodAutoscaler`. Настройка ванильного `prometheus-metrics-adapter` — достаточно трудоемкий процесс, но мы его несколько упростили, определив набор [Custom Resources](cr.html) с разным Scope:
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

С помощью Cluster-scoped-ресурса можно определить метрику глобально, а с помощью Namespaced-ресурса можно ее локально переопределять. [Формат](cr.html) у всех custom resource — одинаковый.

### Применяем кастомные метрики в HPA

После регистрации кастомной метрики на нее можно сослаться. С точки зрения HPA, кастомные метрики бывают двух видов — `Pods` и `Object`. С `Object` все просто — это отсылка к объекту в кластере, который имеет в Prometheus метрики с соответствующими лейблами (`namespace=XXX,ingress=YYY`). Эти лейблы будут подставляться вместо `<<.LabelMatchers>>` в вашем кастомном запросе.

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

C `Pods` сложнее — из ресурса, который масштабирует HPA, будут извлечены все поды и по каждому будут собраны метрики с соответствующими лейблами (`namespace=XXX,pod=YYY-sadiq`,`namespace=XXX,pod=YYY-e3adf` и т. д.). Из этих метрик HPA посчитает среднее и использует для масштабирования. [Пример...](#примеры-с-использованием-кастомных-метрик-типа-pods)

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

Масштабируем Deployment по процентному количеству активных php-fpm-воркеров.

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

### Регистрируем внешние метрики в Kubernetes API

Модуль `prometheus-metrics-adapter` поддерживает механизм `externalRules`, с помощью которого можно определять кастомные PromQL-запросы и регистрировать их как метрики.

В наших инсталляциях мы добавили универсальное правило, которое позволяет создавать свои метрики без внесения настроек в `prometheus-metrics-adapter`, — «любая метрика в Prometheus с именем `kube_adapter_metric_<name>` будет зарегистрирована в API под именем `<name>`». То есть остается либо написать exporter, который будет экспортировать подобную метрику, либо создать [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) в Prometheus, которое будет агрегировать вашу метрику на основе других метрик.

Пример `CustomPrometheusRules`:

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

### Применяем внешние метрики в HPA

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

> Для интеграции с SQS вам понадобится установка экспортера. Для этого нужно завести отдельный «служебный» Git-репозитарий (или, например, использовать «инфраструктурный» репозитарий) и разместить в нем установку этого экспортера, а также скрипт для создания необходимого `CustomPrometheusRules`, таким образом интегрировав кластер. Если же вам нужно настроить автомасштабирование только для одного приложения (особенно живущего в одном пространстве имен), лучше ставить экспортер вместе с этим приложением и воспользоваться `NamespaceMetrics`.

В следующем примере подразумевается, что в Amazon SQS работает очередь `send_forum_message`. Если сообщений в очереди больше 42 — масштабируем. Для получения метрик из Amazon SQS понадобится exporter (например, [sqs-exporter](https://github.com/ashiddo11/sqs-exporter)).

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # Рекомендованное название — prometheus-metrics-adapter-<metric name>.
  name: prometheus-metrics-adapter-sqs-messages-visible
spec:
  groups:
  # Рекомендованный шаблон названия.
  - name: prometheus-metrics-adapter.sqs_messages_visible
    rules:
    # Важно! Префикс 'kube_adapter_metric_' обязателен.
    - record: kube_adapter_metric_sqs_messages_visible
      expr: sum (sqs_messages_visible) by (queue)
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
  - type: External
    external:
      metric:
        # name должен совпадать с CustomPrometheusRules record без префикса 'kube_adapter_metric_'.
        name: sqs_messages_visible
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
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/ingresses/*/rps_1m
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/ingresses/*/mymetric
```

### Как получить значение метрики, созданной через `NamespaceMetric`?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

### Как получить external-метрики?

```shell
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1/namespaces/d8-ingress-nginx/d8_ingress_nginx_ds_cpu_utilization
```
