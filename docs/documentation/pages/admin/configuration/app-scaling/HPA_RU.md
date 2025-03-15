---
title: "Горизонтальное масштабирование"
permalink: ru/admin/configuration/app-scaling/hpa.html
lang: ru
---

## Как работает горизонтальное масштабирование (HPA)

Horizontal Pod Autoscaler (HPA) — это механизм автоматического изменения (вверх или вниз) количества реплик подов (в Deployments, StatefulSets и т. д.) на основе метрик, полученных через Kubernetes API. HPA следит за уровнем нагрузки на приложение, проверяя актуальные метрики (например, CPU, память или кастомные из Prometheus), и при необходимости изменяет количество реплик для поддержания заданного уровня производительности или экономии ресурсов.

HPA используется с `apiVersion: autoscaling/v2`, которая появилась начиная с Kubernetes 1.12 и даёт более гибкое управление поведением при масштабировании.

## Режимы работы HPA

1. Классические метрики (Resource):
   - Масштабирование по потреблению CPU и памяти подов;
   - Используется метрика типа Resource (например, averageUtilization = 70 для CPU).

1. Кастомные метрики (Pods, Object):
   - Позволяют масштабироваться на основе метрик, привязанных к определённым объектам DKP (например, Ingress, Service) или рассчитанных на под (сумма или среднее по всем подам контроллера);
   - Требуют регистрации метрик в Custom Metrics API (см. [Масштабирование по метрикам](http://ru.localhost/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/scaling-by-metrics.html#масштабирование-по-метрикам)).

1. Внешние метрики (External):
   - Используются, когда источник метрик вне кластера (Amazon SQS, облачный Load Balancer, SaaS-сервисы);
   - Требуют регистрации в External Metrics API.

> Рекомендуется в первую очередь использовать классические (Resource) или namespace-scoped кастомные метрики. Cluster-wide и внешние метрики удобно применять, если у вас множество одинаковых сервисов или внешние источники данных.

## Рекомендации HPA

1. При колебаниях метрик оберните метрику в агрегирующую функцию (например, `avg_over_time()`) или увеличьте время стабилизации (`spec.behavior.scaleUp.stabilizationWindowSeconds`), чтобы избежать резкого увеличения количества подов.

1. Используйте классические или namespace-scoped кастомные метрики. Варианты cluster-wide или внешние рассматривайте при большом количестве одинаковых сервисов или внешних SaaS/облачных сервисах.

## Ограничения HPA

1. По умолчанию HPA по-разному обрабатывает масштабирование вверх и вниз:

   - Масштабирование вверх:
     - Происходит незамедлительно (spec.behavior.scaleUp.stabilizationWindowSeconds = 0).
     - Ограничение — за 15 секунд число подов может удвоиться. Если подов было меньше 4, то добавятся 4 новых пода.

   - Масштабирование вниз:
     - Происходит в течение 5 минут (`spec.behavior.scaleUp.stabilizationWindowSeconds = 300`).
     - Собираются «предложения» о новом количестве реплик, и выбирается самое большое, чтобы не было слишком частого уменьшения.
     - Нет ограничений на количество убираемых подов.

1. Необходимость регистрации метрик. Если вы хотите использовать кастомные или внешние метрики, их нужно сначала зарегистрировать в Kubernetes API (Custom/External Metrics API) с помощью `prometheus-metrics-adapter`.

1. Масштабирование только одного контроллера. Нельзя настроить несколько HPA, нацеленных на один и тот же Deployment (или StatefulSet), иначе они будут конфликтовать.

## Как включить или отключить HPA

HPA — это базовый функционал Kubernetes, встроенный в `kube-controller-manager` Kubernetes, и не требует отдельного включения в Deckhouse.

Однако для работы с кастомными и внешними метриками нужен модуль `prometheus-metrics-adapter`. Как включить модуль [см. в документации](http://deckhouse.ru/products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/scaling-by-metrics.html#как-включить-prometheus-metrics-adapter).

## Настройка HPA

Для настройки HPA необходимо:

1. Определить контроллер (Deployment, StatefulSet), который будет масштабироваться.

1. Указать пределы масштабирования (`minReplicas` и `maxReplicas`):

   ```yaml
   minReplicas: 1
   maxReplicas: 10
   ```

1. Сконфигурировать метрики:

   ```yaml
   metrics:
   - type: Resource
     resource:
       name: cpu
       target:
         type: Utilization
         averageUtilization: 70
    ```

1. При необходимости настроить поведение:

   ```yaml
   behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
    # позволяет отложить принятие решения о масштабировании вверх
    # и ограничить скорость прироста подов
   ```

## Примеры настройки HPA

**Пример 1** — классическое масштабирование по CPU и памяти:

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

Масштабирование происходит, если среднее использование CPU/памяти всех подов превышает указанный процент.

**Пример 2** — кастомные метрики (Object) — RabbitMQ-очередь:

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

Если метрика `rmq-queue-forum-messages` (количество сообщений в RabbitMQ) превышает 42, HPA увеличивает число реплик Deployment myconsumer.

**Пример 3** — Внешние метрики (External):

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

Используется метрика `mymetric`, получаемая из внешнего источника (например, SQS). Масштабирование происходит, если значение превышает 100.
