---
title: "Масштабирование платформы и управление узлами"
permalink: ru/admin/update/platform-scaling-and-node-management.html
lang: ru
---

## Введение






## Масштабирование по метрикам

Масштабирование по метрикам — это процесс автоматического или ручного изменения ресурсов (например, количества реплик подов, выделенных CPU/памяти) на основе определенных метрик. Эти метрики могут быть как стандартными (например, использование CPU или памяти), так и кастомными (например, количество запросов в секунду или размер очереди сообщений).

DKP предоставляет возможность масштабирования приложений с использованием HPA- и VPA-автоскейлеров на основе различных метрик. Для этого в кластер устанавливаются следующие интерфейсы API:

- Kubernetes resource metrics API;
- Custom metrics API;
- External metrics API.

Эти интерфейсы получают данные из Prometheus, что позволяет использовать `kubectl top` для получения метрик, применять ресурс `autoscaling/v2` для масштабирования приложений и получать данные через Kubernetes API для других функций, таких как Vertical Pod Autoscaler.

### Доступные метрики для масштабирования

Масштабирование выполняется на основе следующих параметров:

- CPU (пода) — текущее использование процессора.
- Память (пода) — текущее использование оперативной памяти.
- RPS (Ingress) — количество запросов в секунду за 1, 5, 15 минут (rps_1m, rps_5m, rps_15m).
- Среднее потребление CPU (пода) — за 1, 5, 15 минут (cpu_1m, cpu_5m, cpu_15m).
- Среднее потребление памяти (пода) — за 1, 5, 15 минут (memory_1m, memory_5m, memory_15m).
- Любые Prometheus-метрики — возможность использовать любые метрики и запросы на их основе.

DKP использует `k8s-prometheus-adapter` в качестве external API-сервиса, который расширяет возможности Kubernetes API. Когда компонент Kubernetes (например, HPA или VPA) запрашивает информацию о ресурсах, Kubernetes API перенаправляет запрос в адаптер. Адаптер определяет способ расчета метрики на основе конфигурации и делает запрос в Prometheus.

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

## Вертикальное масштабирование (VPA)

Vertical Pod Autoscaler (VPA) — это сервис, который помогает автоматически настраивать resource requests для контейнеров, когда точные значения этих параметров неизвестны. При использовании VPA и включении соответствующего режима работы, resource requests устанавливаются на основе фактического потребления ресурсов, полученных из Prometheus. Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, не внося изменений автоматически.

VPA поддерживает следующие режимы работы:

- **Auto** (по умолчанию) — режимы Auto и Recreate выполняют одинаковую задачу.
- **Recreate** — в этом режиме VPA может изменять ресурсы у работающих подов, перезапуская их. В случае одного пода (replicas: 1) это приведет к недоступности сервиса на время перезапуска. В этом режиме VPA не пересоздает поды, если они были созданы без контроллера.
- **Initial** — ресурсы подов изменяются только при их создании, но не в процессе работы.
- **Off** — VPA не меняет автоматически ресурсы. В этом случае можно просмотреть рекомендации по ресурсам, которые предоставляет VPA (с помощью команды `kubectl describe vpa`).

Ограничения VPA:

- Обновление ресурсов работающих подов — это экспериментальная функция. При изменении resource requests пода, под пересоздается, что может привести к его запуску на другом узле.
- VPA не рекомендуется использовать совместно с HPA для CPU и памяти в данный момент. Однако VPA можно применять с HPA для custom/external metrics.
- VPA реагирует на большинство событий out-of-memory, но не гарантирует реакцию (подробности нужно искать в документации).
- Производительность VPA не была протестирована на крупных кластерах.
- Рекомендации VPA могут превышать доступные ресурсы кластера, что может привести к тому, что поды окажутся в состоянии Pending.
- Использование нескольких VPA-ресурсов для одного пода может вызвать неопределенное поведение.
- При удалении VPA или отключении его (режим Off) изменения, внесенные VPA, сохраняются в последнем измененном виде. Это может привести к путанице, когда в Helm указаны одни ресурсы, в контроллере — другие, но на самом деле у подов будут другие ресурсы, что создаст впечатление, что они появились «непонятно откуда».

Важно! При использовании VPA рекомендуется настраивать Pod Disruption Budget.

VPA состоит из 3 компонентов:

- Recommender — мониторит настоящее (делая запросы в Metrics API, который реализован в модуле prometheus-metrics-adapter) и прошлое потребление ресурсов (делая запросы в Trickster перед Prometheus) и предоставляет рекомендации по CPU и памяти для контейнеров.
- Updater — проверяет, что у подов с VPA выставлены корректные ресурсы, если нет — убивает эти поды, чтобы контроллер пересоздал поды с новыми resource requests.
- Admission Plugin — задает resource requests при создании новых подов (контроллером или из-за активности Updater’а).

При изменении ресурсов компонентом Updater это происходит с помощью Eviction API, поэтому учитываются Pod Disruption Budget для обновляемых подов.

### Рекомендации VPA

После создания ресурса VerticalPodAutoscaler посмотреть рекомендации VPA можно с помощью команды:

```console
kubectl describe vpa my-app-vpa
```

В секции status будут такие параметры:

- Target — количество ресурсов, которое будет оптимальным для пода (в пределах resourcePolicy);
- Lower Bound — минимальное рекомендуемое количество ресурсов для более или менее (но не гарантированно) хорошей работы приложения;
- Upper Bound — максимальное рекомендуемое количество ресурсов. Скорее всего, ресурсы, выделенные сверх этого значения, идут в мусорку и совсем никогда не нужны приложению;
- Uncapped Target — рекомендуемое количество ресурсов в самый последний момент, то есть данное значение считается на основе самых крайних метрик, не смотря на историю ресурсов за весь период.

### Настройка VPA

1. Создайте конфигурации модуля VPA.

   Для настройки VPA  нужно создать файл конфигурации для модуля. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     version: 1
     enabled: true
     settings:
       nodeSelector:
         node-role/example: ""
       tolerations:
       - key: dedicated
         operator: Equal
         value: example
      ```

1. Примените файл конфигурации для VPA с помощью `kubectl apply -f <имя файла>`.

### Работа VPA с лимитами

1. Пример 1. В кластере имеется:

   - Объект VPA:

     ```yaml
     apiVersion: autoscaling.k8s.io/v1
     kind: VerticalPodAutoscaler
     metadata:
       name: test2
     spec:
       targetRef:
         apiVersion: "apps/v1"
         kind: Deployment
         name: test2
       updatePolicy:
         updateMode: "Initial"
     ```

   - Под с ресурсами:

     ```yaml
     resources:
     limits:
       cpu: 2
     requests:
       cpu: 1
     ```

     Если контейнер будет потреблять 1 CPU, VPA порекомендует 1,168 CPU. В данном случае, соотношение между запросами и лимитами будет равно 100%. При пересоздании пода VPA изменит ресурсы на следующие:

     ```yaml
     resources:
     limits:
       cpu: 2336m
     requests:
       cpu: 1168m
     ```

1. Пример 2. В кластере имеется:

   - VPA:

     ```yaml
     apiVersion: autoscaling.k8s.io/v1
     kind: VerticalPodAutoscaler
     metadata:
       name: test2
     spec:
       targetRef:
         apiVersion: "apps/v1"
         kind: Deployment
         name: test2
       updatePolicy:
         updateMode: "Initial"
     ```

   - Под с ресурсами:

     ```yaml
     resources:
     limits:
       cpu: 1
     requests:
       cpu: 750m
     ```

     В данном случае соотношение реквестов и лимитов будет 25%. Если VPA порекомендует 1,168 CPU, ресурсы контейнера будут изменены на:

     ```yaml
     resources:
      limits:
        cpu: 1557m
      requests:
        cpu: 1168m
     ```

     Если вам необходимо ограничить максимальное количество ресурсов, которое может быть заданно для лимитов контейнера, необходимо использовать в спецификации VPA-объекта `maxAllowed` или использовать [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) объект Kubernetes.

### Примеры настройки VPA

1. Пример минимального ресурса VerticalPodAutoscaler:

   ```yaml
   apiVersion: autoscaling.k8s.io/v1
   kind: VerticalPodAutoscaler
   metadata:
    name: my-app-vpa
   spec:
     targetRef:
       apiVersion: "apps/v1"
       kind: StatefulSet
       name: my-app
   ```

1. Пример полного ресурса VerticalPodAutoscaler:

   ```yaml
   apiVersion: autoscaling.k8s.io/v1
   kind: VerticalPodAutoscaler
   metadata:
     name: my-app-vpa
   spec:
     targetRef:
       apiVersion: "apps/v1"
       kind: Deployment
       name: my-app
     updatePolicy:
       updateMode: "Auto"
     resourcePolicy:
       containerPolicies:
       - containerName: hamster
         minAllowed:
           memory: 100Mi
           cpu: 120m
         maxAllowed:
           memory: 300Mi
           cpu: 350m
         mode: Auto
    ```

## Автомасштабирование

Настройка Cluster Autoscaler для групп узлов.

Приоритеты групп узлов при автомасштабировании.

Примеры конфигурации для автомасштабирования.

## Перераспределение нагрузки и вытеснение подов

Каждые 15 минут планировщик DKP анализирует состояние кластера и выполняет вытеснение подов, соответствующих условиям, описанным в активных стратегиях. Вытесненные поды вновь проходят процесс планирования с учетом текущего состояния кластера. Это позволяет перераспределить рабочие нагрузки в соответствие с выбранной стратегией.

Планировщик основан на проекте [descheduler](https://github.com/kubernetes-sigs/descheduler).

- Планировщик DKP может учитывать класс приоритета пода (параметр `spec.priorityClassThreshold`), ограничивая работу только подами, у которых класс приоритета ниже заданного порога.

- Планировщик не вытесняет под в следующих случаях:
  - под находится в пространстве имен `d8-*` или `kube-system`;
  - под имеет priorityClassName `system-cluster-critical` или `system-node-critical`;
  - под связан с локальным хранилищем;
  - под связан с DaemonSet;
  - вытеснение пода нарушит Pod Disruption Budget (PDB);
  - нет доступных узлов для запуска вытесненного пода.

- Поды с классом приоритета `Best effort` вытесняются раньше, чем поды с классами `Burstable` и `Guaranteed`.

Для фильтрации подов и узлов планировщик использует механизм `labelSelector` Kubernetes:

- podLabelSelector — ограничивает поды по меткам;
- namespaceLabelSelector — фильтрует поды по пространствам имен.
- nodeLabelSelector — выбирает узлы по меткам.

### Стратегии планировщика

1. **HighNodeUtilization** — rонцентрирует нагрузку на меньшем числе узлов. Требует настройку планировщика и включение автоматического масштабирования. Чтобы использовать HighNodeUtilization, необходимо явно указать профиль планировщика high-node-utilization для каждого пода (этот профиль не может быть установлен как профиль по умолчанию).
Стратегия определяет недостаточно нагруженные узлы и вытесняет с них поды, чтобы распределить их компактнее, на меньшем числе узлов.
Недостаточно нагруженный узел — узел, использование ресурсов которого меньше всех пороговых значений, заданных в секции параметров [`strategies.highNodeUtilization.thresholds`](todo).

Стратегия включается параметром [`spec.strategies.highNodeUtilization.enabled`](todo).

В GKE нельзя настроить конфигурацию планировщика по умолчанию, но можно использовать стратегию `optimize-utilization` или развернуть второй пользовательский планировщик.

Использование ресурсов узла учитывает extended-ресурсы и рассчитывается на основе запросов и лимитов подов (requests and limits), а не их фактического потребления. Такой подход обеспечивает согласованность с работой kube-scheduler, который использует аналогичный принцип при размещении подов на узлах. Это означает, что метрики использования ресурсов, отображаемые Kubelet (или командами вроде kubectl top), могут отличаться от расчетных показателей, так как Kubelet и связанные инструменты отображают данные о реальном потреблении ресурсов.

1. **LowNodeUtilization** — более равномерно нагружает узлы. Стратегия выявляет недостаточно нагруженные узлы и вытесняет поды с других, избыточно нагруженных узлов. Стратегия предполагает, что пересоздание вытесненных подов произойдет на недостаточно нагруженных узлах (при обычном поведении планировщика).

Недостаточно нагруженный узел — узел, использование ресурсов которого меньше всех пороговых значений, заданных в секции параметров [`strategies.lowNodeUtilization.thresholds`](todo).

Избыточно нагруженный узел — узел, использование ресурсов которого больше хотя бы одного из пороговых значений, заданных в секции параметров [`strategies.lowNodeUtilization`.targetThresholds.](todo)

Узлы с использованием ресурсов в диапазоне между thresholds и targetThresholds считаются оптимально используемыми. Поды на таких узлах вытесняться не будут.

Стратегия включается параметром [`spec.strategies.lowNodeUtilization.enabled`](todo).

Использование ресурсов узла учитывает extended-ресурсы и рассчитывается на основе запросов и лимитов подов (requests and limits), а не их фактического потребления. Такой подход обеспечивает согласованность с работой kube-scheduler, который использует аналогичный принцип при размещении подов на узлах. Это означает, что метрики использования ресурсов, отображаемые Kubelet (или командами вроде kubectl top), могут отличаться от расчетных показателей, так как Kubelet и связанные инструменты отображают данные о реальном потреблении ресурсов.

1. **RemoveDuplicates** — предотвращает запуск нескольких подов одного контроллера (ReplicaSet, ReplicationController, StatefulSet) или заданий (Job) на одном узле.
Стратегия следит за тем, чтобы на одном узле не находилось больше одного пода ReplicaSet, ReplicationController, StatefulSet или подов одного задания (Job). Если таких подов два или больше, модуль вытесняет лишние поды, чтобы они лучше распределились по кластеру.

Описанная ситуация может возникнуть, если некоторые узлы кластеры вышли из строя по каким-либо причинам, и поды с них были перемещены на другие узлы. Как только вышедшие из строя узлы снова станут доступны для приема нагрузки, эту стратегию можно будет использовать для выселения дублирующих подов с других узлов.

Стратегия включается параметром [`strategies.removeDuplicates.enabled`](todo).

1. **RemovePodsViolatingInterPodAntiAffinity** — вытесняет поды, нарушающие [правила node affinity](todo). Стратегия гарантирует, что все поды, которые нарушают правила node affinity, в конечном счете будут удалены с узлов.

По сути, в зависимости от настроек параметра strategies.removePodsViolatingNodeAffinity.nodeAffinityType, стратегия превращает правило requiredDuringSchedulingIgnoredDuringExecution node affinity пода в правило requiredDuringSchedulingRequiredDuringExecution, а правило preferredDuringSchedulingIgnoredDuringExecution в правило preferredDuringSchedulingPreferredDuringExecution.

Пример для nodeAffinityType: requiredDuringSchedulingIgnoredDuringExecution. Есть под, который был назначен на узел, соответствующий правилу requiredDuringSchedulingIgnoredDuringExecution node affinity на момент размещения. Если со временем этот узел перестанет удовлетворять правилу node affinity, и если появится другой доступный узел, соответствующий этому правилу, стратегия вытеснит под с узла, на который он был изначально назначен.

Пример для nodeAffinityType: preferredDuringSchedulingIgnoredDuringExecution. Есть под, который был назначен на узел, т.к. на момент размещения отсутствовали другие узлы, удовлетворяющие правилу preferredDuringSchedulingIgnoredDuringExecution node affinity. Если со временем в кластере появится доступный узел, соответствующий этому правилу, стратегия вытеснит под с узла, на который он был изначально назначен.

Стратегия включается параметром strategies.removePodsViolatingNodeAffinity.enabled.

### Примеры стратегий

Пример стратегии LowNodeUtilization:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: low-node-utilization
spec:
  strategies:
    lowNodeUtilization:
      thresholds:
        cpu: 20
      targetThresholds:
        cpu: 50
```

Пример стратегии HighNodeUtilization:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: Descheduler
metadata:
  name: high-node-utilization
spec:
  strategies:
    highNodeUtilization:
      thresholds:
        cpu: 50
        memory: 50
```

## Управление узлами

Как правило, кластеры в production-системах состоят из более чем одного узла. Разные узлы предназначены для решения различных задач: например, на схеме ниже можно увидеть несколько мастер- и системных узлов, узлы для фронтенда, а также рабочие узлы для разработки и продакшена. Поскольку в Kubernetes нет понятия «группа узлов», мы объединяем их путём добавления аннотаций и лейблов, которые позволяют идентифицировать узлы.

Если такой кластер масштабировать, например добавить в него серверы под конкретный проект, могут возникнуть трудности: придётся настраивать эксклюзивный доступ для приложений проекта на каждый новый сервер. Сильно упростить этот процесс может автоматическая настройка серверов. В DKP это возможно благодаря встроенному механизму объединения узлов в группы.

Управление узлами осуществляется с помощью модуля node-manager, основные функции которого:

1. Управление несколькими узлами как связанной группой (NodeGroup):
   - Возможность определить метаданные, которые наследуются всеми узлами группы.
   - Мониторинг группы узлов как единой сущности (группировка узлов на графиках по группам, группировка алертов о недоступности узлов, алерты о недоступности N узлов или N% узлов группы).

1. Систематическое прерывание работы узлов — Chaos Monkey. Предназначено для верификации отказоустойчивости элементов кластера и запущенных приложений.

1. Установка/обновление и настройка ПО узла (containerd, kubelet и др.), подключение узла в кластер:
   - Установка операционной системы (смотри список поддерживаемых ОС) вне зависимости от типа используемой инфраструктуры (в любом облаке или на любом железе).
   - Базовая настройка операционной системы (отключение автообновления, установка необходимых пакетов, настройка параметров журналирования и т. д.).
   - Настройка nginx (и системы автоматического обновления перечня upstream’ов) для балансировки запросов от узла (kubelet) по API-серверам.
   - Установка и настройка CRI containerd и Kubernetes, включение узла в кластер.
   - Управление обновлениями узлов и их простоем (disruptions):
     - Автоматическое определение допустимой минорной версии Kubernetes группы узлов на основании ее настроек (указанной для группы kubernetesVersion), версии по умолчанию для всего кластера и текущей действительной версии control plane (не допускается обновление узлов в опережение обновления control plane).
     - Из группы одновременно производится обновление только одного узла и только если все узлы группы доступны.
   - Два варианта обновлений узлов:
     - обычные — всегда происходят автоматически;
     - требующие disruption (например, обновление ядра, смена версии containerd, значительная смена версии kubelet и пр.) — можно выбрать ручной или автоматический режим. В случае, если разрешены автоматические disruptive-обновления, перед обновлением производится drain узла (можно отключить).

   - Мониторинг состояния и прогресса обновления.

1. Масштабирование кластера.
   - Автоматическое масштабирование. Доступно при использовании поддерживаемых облачных провайдеров (подробнее) и недоступно для статических узлов. Облачный провайдер в автоматическом режиме может создавать или удалять виртуальные машины, подключать их к кластеру или отключать.
   - Поддержание желаемого количества узлов в группе. Доступно как для облачных провайдеров, так и для статических узлов (при использовании Cluster API Provider Static).

1. Управление Linux-пользователями на узлах.

Узлы в группе имеют общие параметры и настраиваются автоматически в соответствии с параметрами группы. Deckhouse масштабирует группы, добавляя, исключая и обновляя ее узлы. Допускается иметь в одной группе как облачные, так и статические узлы (серверы bare metal, виртуальные машины). Это позволяет получать узлы на физических серверах, которые могут масштабироваться за счет облачных узлов (гибридные кластеры).

Работа в облачной инфраструктуре осуществляется с помощью поддерживаемых облачных провайдеров. Если поддержки необходимой облачной платформы нет, возможно использование ее ресурсов в виде статических узлов.

Работа со статическими узлами (например, серверами bare metal) выполняется с помощью в провайдера CAPS (Cluster API Provider Static).

Поддерживается работа со следующими сервисами Managed Kubernetes (может быть доступен не весь функционал сервиса):

- Google Kubernetes Engine (GKE);
- Elastic Kubernetes Service (EKS).

### Типы узлов

Типы узлов, с которыми возможна работа в группах узлов (ресурс NodeGroup):

- CloudEphemeral — узлы автоматически заказываются, создаются и удаляются в настроенном облачном провайдере.
- CloudPermanent — узлы отличаются тем, что их конфигурация берется не из custom resource nodeGroup, а из специального ресурса <PROVIDER>ClusterConfiguration (например,  AWSClusterConfiguration для AWS). Также важное отличие узлов в том, что для применения их конфигурации необходимо выполнить dhctl converge (запустив инсталлятор Deckhouse). Примером CloudPermanent-узла облачного кластера является master-узел кластера.
- CloudStatic — узел, созданный вручную (статический узел), размещенный в том же облаке, с которым настроена интеграция у одного из облачных провайдеров. На таком узле работает CSI и такой узел управляется cloud-controller-manager'ом. Объект Node кластера обогащается информацией о зоне и регионе, в котором работает узел. Также при удалении узла из облака соответствующий ему Node-объект будет удален в кластере.
- Static — статический узел, размещенный на сервере bare metal или виртуальной машине. В случае облака, такой узел не управляется cloud-controller-manager'ом, даже если включен один из облачных провайдеров. Подробнее про работу со статическими узлами…

### Группы узлов

Группировка и управление узлами как связанной группой означает, что все узлы группы будут иметь одинаковые метаданные, взятые из ресурса NodeGroup.

В группе узлов можно задавать шаблоны, которые позволяют указать настройки (лейблы, тейнты, аннотации, другие элементы конфигурации) в одном месте, а затем применить их ко всем узлам этой группы.

В качестве узла кластера могут выступать три типа серверов: static — физический сервер с установленной на него ОС, static — виртуальная машина на гипервизоре и cloud — виртуальная машина в облаке.

Логика добавления static- и cloud-узлов различается: для cloud доступна бóльшая степень автоматизации, чем для static. Для последних, как минимум, необходимо вручную заказывать виртуальные машины и устанавливать на них ОС.

Для групп узлов доступен мониторинг:

- с группировкой параметров узлов на графиках группы;
- с группировкой алертов о недоступности узлов;
- с алертами о недоступности N узлов или N% узлов группы и т


Создание и управление группами узлов.

Настройка шаблонов для узлов (лейблы, тейнты, аннотации).

Примеры конфигурации NodeGroup для различных типов узлов.

### Добавление узлов в кластер

#### Добавление узлов в bare-metal-кластер

Прежде чем добавлять узлы в bare-metal-кластер, необходимо заранее подготовить серверы. Во-первых, на них нужно установить [поддерживаемую  операционную систему](todo). В кластере допускается одновременно использовать разные ОС. Во-вторых, необходимо убедиться, что на добавляемых серверах есть сетевая связность с мастер-узлами, а в конфигурации StaticClusterConfiguration прописаны подсети адресов, которые используются в интерфейсах.

Добавить серверы в кластер можно вручную и автоматически — рассмотрим эти способы по очереди.

1. Ручной способ.

Сначала необходимо создать в кластере объект NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

В спецификации этого ресурса укажем тип узлов Static. Для всех объектов NodeGroup в кластере автоматически будет создан скрипт bootstrap.sh, с помощью которого узлы добавляются в группы. Когда узлы добавляются вручную, необходимо скопировать этот скрипт на сервер и выполнить.

Скрипт можно получить в консоли администратора на вкладке «Группы узлов → Скрипты» или командой kubectl:

```console
kubectl -n d8-cloud-instance-manager get secrets manual-bootstrap-for-worker -ojsonpath="{.data.bootstrap\.sh}"
```

Здесь worker — имя созданной ранее группы узлов.

[!image](...)

Скрипт нужно раскодировать из Base64, а затем выполнить от root.

[!image](...)

Когда скрипт выполнится, сервер добавится в кластер в качестве узла той группы, для которой был использован скрипт.

1. Автоматический способ.

Чтобы автоматически добавить узлы в кластер, помимо установки ОС и настройки сети на сервере, необходимо создать пользователя ОС — с его помощью DKP будет управлять этим сервером по SSH.
Для подключения по SSH используется пара ключей. Приватный ключ из этой пары необходимо сохранить в специальном объекте SSHCredentials в кластере DKP:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SSHCredentials
metadata:
  name: static-nodes
spec:
  privateSSHKey: |
  LS0tLS1CRUdJlhrdG...................VZLS0tLS0K
  sshPort: 22
  sudoPassword: password
  user: ubuntu
```

В этом же объекте нужно указать имя пользователя и порт для подключения по SSH. Чтобы управлять сервером, DKP использует sudo для повышения привилегии до root. Если для sudo нужен пароль, его также необходимо указать в SSHCredentials.

[!image](...)

Далее создадим объекты StaticInstance, каждый из которых соответствует одному из серверов.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: static-0
spec:
  address: 192.168.1.10
  credentialsRef:
    apiVersion: deckhouse.io/v1alpha1
    kind: SSHCredentials
    name: static-nodes
```

В этом ресурсе следует указать IP-адрес сервера, который передаётся под управление DKP, и ссылку на объект SSHCredentials, в котором прописаны доступы на этот сервер.

Обратите внимание: под каждый сервер необходимо создавать отдельный ресурс StaticInstance, но можно использовать одни и те же SSHCredentials для доступа на разные серверы.

Группа для автоматического добавления узлов выглядит так:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    count: 3
    labelSelector:
      matchExpressions: []
      matchLabels:
        static-node: auto
```

Здесь добавляются параметры, которые описывают использование StaticInstances: count указывает, сколько узлов будет добавлено в эту группу; в labelSelector прописываются правила для создания выборки узлов.

После того как группа узлов будет создана, появится скрипт для добавления серверов в эту группу. DKP будет ждать, пока в кластере появится необходимое количество объектов StaticInstance, которые подходят под выборку по лейблам.

Как только такой объект появится, DKP получит из созданных ранее манифестов IP-адрес сервера и параметры для подключения по SSH, подключится к серверу и выполнит на нём скрипт bootstrap.sh. После этого сервер добавится в заданную группу в качестве узла.










Ручное добавление: Использование скрипта bootstrap.sh.

Автоматическое добавление: Использование Cluster API Provider Static (CAPS).

Примеры добавления узлов в bare-metal и cloud-кластеры.

### Конфигурация узлов

NodeGroupConfiguration: Добавление пользовательских скриптов для настройки узлов.

NodeUser: Управление пользователями на узлах.

Примеры настройки CRI, обновления ядра, добавления сертификатов.

### Обновление узлов

Обычные и disruptive-обновления.

Настройка параметров обновлений (maxConcurrent, disruptions).

### Мониторинг и логирование

Метрики Prometheus для групп узлов.

Просмотр логов сервиса bashible.

### Примеры конфигураций

