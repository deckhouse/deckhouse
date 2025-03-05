---
title: "Горизонтальное масштабирование"
permalink: ru/admin/configuration/app-scaling/hpa.html
lang: ru
---

## Горизонтальное масштабирование (HPA)

HPA автоматически регулирует количество реплик подов в зависимости от нагрузки. DKP использует метрики, доступные через Kubernetes API, и может масштабировать приложения на основе:

- [Стандартных метрик (Resource)](todo) — используются для масштабирования по потреблению CPU и памяти.

  Пример:

  ```yaml
  metrics:
   - type: Resource
     resource:
       name: cpu
       target:
         type: Utilization
         averageUtilization: 70
  ```yaml

- [Кастомных namespace-scoped-метрик](todo) — используются, если у вас одно приложение, и источник метрик находится внутри пространства имён и связан с одним из объектов (например, Deployment, Service).

  Пример:

  ```yaml
   metrics:
   - type: Object
     object:
       describedObject:
         apiVersion: v1
         kind: Service
         name: my-service
       metric:
         name: my-custom-metric
       target:
         type: AverageValue
         averageValue: 10
  ```

- [Кастомных cluster-wide-метрик](todo) — используются, если у вас много приложений используют одинаковую метрику, источник которой находится в пространстве имён приложения. Подходит для общих инфраструктурных компонентов, выделенных в отдельный деплой.

  > **Важно**. Рекомендуется использовать классические метрики или кастомные метрики, определяемые в пространстве имён. Конфигурацию масштабирования следует хранить в репозитории приложения. Варианты с cluster-wide-метриками и внешними метриками стоит рассматривать только в случае, если у вас есть большая коллекция идентичных микросервисов

- [Внешних метрик (External)](todo) — используются, если источник метрики не привязаны к пространству имён приложения (например, метрики облачного провайдера или внешнего SaaS-сервиса).

  Пример:

  ```yaml
  metrics:
   - type: External
     external:
       metric:
         name: my-external-metric
       target:
         type: Value
         value: 100
  ```

   DKP поддерживает механизм `externalRules`, с помощью которого можно определять кастомные PromQL-запросы и регистрировать их как метрики.

   В примерах инсталляций добавлено универсальное правило, которое позволяет создавать собственные метрики — «любая метрика в Prometheus с именем `kube_adapter_metric_<name>` будет зарегистрирована в API под именем <name>». После чего, остается написать экспортер (exporter), который будет экспортировать подобную метрику, или создать правило `recording rule` в Prometheus, которое будет агрегировать вашу метрику на основе других метрик.

   Пример пользовательских правил Prometheus для метрики `mymetric`:

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
         # Запрос, результаты которого попадут в итоговую метрику.
         expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress)
    ```

    После регистрации внешней метрики на нее можно сослаться. Пример:

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

Кастомные метрики необходимо регистрировать в API `/apis/custom.metrics.k8s.io/`, что выполняется через `prometheus-metrics-adapter`, который также реализует это API. Эти метрики можно использовать в объекте `HorizontalPodAutoscaler`. Процесс настройки стандартного `prometheus-metrics-adapter` происходит с помощью [кастомных ресурсов](todo) с разными областями применения:

- **Namespaced**:
  - `ServiceMetric`;
  - `IngressMetric`;
  - `PodMetric`;
  - `DeploymentMetric`;
  - `StatefulsetMetric`;
  - `NamespaceMetric`;
  - `DaemonSetMetric` (недоступен пользователям).

- **Cluster**:
  - `ClusterServiceMetric` (недоступен пользователям);
  - `ClusterIngressMetric` (недоступен пользователям);
  - `ClusterPodMetric` (недоступен пользователям);
  - `ClusterDeploymentMetric` (недоступен пользователям);
  - `ClusterStatefulsetMetric`(недоступен пользователям);
  - `ClusterDaemonSetMetric` (недоступен пользователям).

С помощью ресурсов с кластерным уровнем можно задать глобальное определение метрики, а с помощью ресурсов с `Namespaced` уровнем — переопределить её локально. Формат для всех кастомных ресурсов одинаков.

### Особенности поведения HPA

{% alert level="warning" %}
По умолчанию HPA использует разные подходы при масштабировании: масштабирование вверх, масштабирование вниз, стабилизацию при колебаниях метрик.
{% endalert %}

- **Масштабирование вверх** — если метрики указывают на необходимость увеличения количества реплик, масштабирование происходит незамедлительно (`spec.behavior.scaleUp.stabilizationWindowSeconds = 0`). Ограничение — скорость прироста: за 15 секунд количество подов может удвоиться. Если подов меньше 4, будет добавлено 4 новых пода.

- **Масштабирование вниз** — если метрики указывают на необходимость уменьшения количества реплик, масштабирование происходит в течение 5 минут (`spec.behavior.scaleDown.stabilizationWindowSeconds = 300`). В течение этого времени собираются предложения о новом количестве реплик, и выбирается самое большое значение. Это позволяет избежать слишком частого изменения количества подов. Нет ограничений на количество удаляемых подов за один раз.

- **Стабилизация при колебаниях метрик** — если метрики нестабильны и происходят резкие скачки, это может привести к нежелательному увеличению количества реплик. Для решения этой проблемы можно использовать следующие подходы:

  - Агрегирование метрик: обернуть метрику агрегирующей функцией (например, `avg_over_time()`), если метрика определена через PromQL-запрос. Это сглаживает колебания.
  - Увеличение времени стабилизации: задать параметр `spec.behavior.scaleUp.stabilizationWindowSeconds` в ресурсе HorizontalPodAutoscaler. В течение указанного периода будут собираться предложения об увеличении количества реплик, и будет выбрано самое скромное предложение. Это эквивалентно применению функции `min_over_time(<stabilizationWindowSeconds>)`, но только для масштабирования вверх.
  - Ограничение скорости прироста: использовать политики `spec.behavior.scaleUp.policies`, чтобы контролировать, насколько быстро может увеличиваться количество реплик.

### Настройка HPA

Для настройки HPA необходимо:

1. Определить масштабируемый объект (`.spec.scaleTargetRef`)`:

   ```yaml
   scaleTargetRef:
     apiVersion: apps/v1
     kind: Deployment
     name: my-app
   ```

1. Задать диапазон масштабирования (`.spec.minReplicas`, `.spec.maxReplicas`):

   ```yaml
   minReplicas: 1
   maxReplicas: 10
   ```

1. Определить метрики для масштабирования (`.spec.metrics`):

   ```yaml
   metrics:
   - type: Resource
     resource:
       name: cpu
       target:
         type: Utilization
         averageUtilization: 70
   ```

### Примеры настройки HPA

1. Пример HPA для масштабирования по базовым метрикам из `metrics.k8s.io`: CPU и памяти подов:

  ```yaml
  apiVersion: autoscaling/v2
   kind: HorizontalPodAutoscaler
   metadata:
     name: app-hpa
     namespace: app-prod
   spec:
     # Указывается контроллер, который нужно масштабировать (ссылка на Deployment или StatefulSet).
     scaleTargetRef:
       apiVersion: apps/v1
       kind: Deployment
       name: app
     # Границы масштабирования контроллера.
     minReplicas: 1
     maxReplicas: 10
     # Если для приложения характерны кратковременные скачки потребления CPU.
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

  {% alert level="info" %}
  Параметр `averageUtilization` указывает целевой процент использования ресурсов.
  Для CPU и памяти доступен только тип `Utilization`.
  {% endalert %}

1. Пример использования кастомных метрик с размером очереди RabbitMQ:

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
     # Указывается контроллер, который нужно масштабировать (ссылка на Deployment или StatefulSet).
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

  {% alert level="info" %}
  В представленном примере рассматривается очередь `send_forum_message` в RabbitMQ, для которого зарегистрирован сервис `rmq`. Если количество сообщений в этой очереди превышает 42, выполняется масштабирование.
  {% endalert %}

1. Пример использования нестабильной кастомной метрики:

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

   {% alert level="info" %}
   В представленном примере рассматривается очередь `send_forum_message` в RabbitMQ, для которого зарегистрирован сервис `rmq`. Если количество сообщений в этой очереди превышает 42, выполняется масштабирование. Используется MQL-функция `avg_over_time()`, чтобы усреднить метрику.
   {% endalert %}

1. Примеры с использованием кастомных метрик типа `Pods`:

   - Пример масштабирования воркеров по процентному количеству активных php-fpm-воркеров:

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
       # Указывается контроллер, который нужно масштабировать (ссылка на Deployment или StatefulSet).
       scaleTargetRef:
         apiVersion: apps/v1
         kind: Deployment
         name: mybackend
       minReplicas: 1
       maxReplicas: 5
       metrics:
       # Указание HPA обойти все поды Deployment'а и собрать с них метрики.
       - type: Pods
         # Указывать describedObject (в отличие от type: Object) не нужно.
         pods:
           metric:
             # Кастомная метрика, зарегистрированная с помощью ресурса PodMetric.
             name: php-fpm-active-workers
           target:
             # Для метрик с type: Pods можно использовать только AverageValue.
             type: AverageValue
             # Масштабирование, если среднее значение метрики у всех подов Deployment'а больше 5.
             averageValue: 5
       ```

       {% alert level="info" %}
       В представленом примере среднее количество php-fpm-воркеров в Deployment `mybackend` не больше 5.
       {% endalert %}

   - Пример масштабирования Deployment по процентному количеству активных php-fpm-воркеров:

     ```yaml
     ---
     apiVersion: deckhouse.io/v1beta1
     kind: PodMetric
     metadata:
       name: php-fpm-active-worker
     spec:
       # Процент активных php-fpm-воркеров.
       query: round(sum by(<<.GroupBy>>) (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) / sum by(<<.GroupBy>>) (phpfpm_processes_total{<<.LabelMatchers>>}) * 100)
     ---
     kind: HorizontalPodAutoscaler
     apiVersion: autoscaling/v2
     metadata:
       name: {{ .Chart.Name }}-hpa
     spec:
       # Указывается контроллер, который нужно масштабировать (ссылка на Deployment или StatefulSet).
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

1. Пример экспортера (например, `sqs-exporter`) для получения метрик из Amazon SQS:

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

    {% alert level="info" %}
    В предоставленном примере в Amazon SQS работает очередь send_forum_message и выполняется масштабирование при количестве сообщений в этой очереди больше 42.
    {% endalert %}

   Чтобы установить экспортер для интеграции с SQS:

   - Создайте отдельный “служебный” репозиторий Git (или, к примеру, можно использовать “инфраструктурный” репозиторий).
   - Разместите в нем инсталляцию экспортера и сценарий для создания требуемого `CustomPrometheusRules`.
   - Готово, вы объединили кластер. Если необходимо настроить автомасштабирование только для одного приложения (в одном пространстве имен), лучше ставить экспортер вместе с этим приложением и воспользоваться `NamespaceMetrics`.

### Получение значений метрик

Для получения списка кастомных метрик используйте команду:

```console
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

Для получения значений метрик, привязанных к объектам используйте команду:

```console
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
```

Для получения значений метрик, созданных через `NamespaceMetric` используйте команду:

```console
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

Для получения внешних (external) метрик используйте команду:

```console
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1
```
