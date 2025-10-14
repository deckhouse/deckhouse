---
title: "Горизонтальное масштабирование"
permalink: ru/admin/configuration/app-scaling/hpa.html
description: "Настройка Horizontal Pod Autoscaler (HPA) в Deckhouse Kubernetes Platform. Автоматическое масштабирование подов на основе CPU, памяти и пользовательских метрик для оптимального использования ресурсов."
lang: ru
---

## Как работает горизонтальное масштабирование (HPA)

Horizontal Pod Autoscaler (HPA) — это механизм автоматического изменения (вверх или вниз) количества реплик подов (в Deployments или StatefulSets) на основе метрик, полученных через Kubernetes API. HPA следит за уровнем нагрузки на приложение, проверяя актуальные метрики (например, CPU, память или дополнительные из Prometheus), и при необходимости изменяет количество реплик для поддержания заданного уровня производительности или экономии ресурсов.

## Доступные типы метрик для HPA

Горизонтальное масштабирование в DKP может выполняться по любым доступным метрикам, например:

1. [По потреблению CPU и памяти подов](hpa.html#масштабирование-по-cpu-и-памяти).
   - Настраивается с помощью ресурса [HorizontalPodAutoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/).  
   Например, можно указать метрику типа Resource с `averageUtilization = 70` для CPU, чтобы при среднем использовании 70% масштабировать приложение вверх.

1. [По метрикам объектов DKP](hpa.html#масштабирование-по-метрикам-объектов) (Ingress, Service) или по метрикам самих подов (сумма или среднее по всем подам одного контроллера).
   - Позволяют масштабироваться на основе метрик, привязанных к объектам DKP (например, Ingress, Service), или рассчитанных на под (сумма или среднее по всем подам контроллера). Используются ресурсы ServiceMetric для сервисов и IngressMetric для Ingress.

1. [По любым другим метрикам, включая внешние данные](hpa.html#масштабирование-на-основе-внешних-данных) (метрики Amazon SQS, облачных балансировщиков, SaaS-сервисов и т. п.).
   - Используются, когда источник метрик вне кластера (Amazon SQS, облачный Load Balancer, SaaS-сервисы). Настраивается с помощью ресурса [CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules).

## Рекомендации HPA

При колебаниях метрик оберните метрику в агрегирующую функцию (например, `avg_over_time()`) или увеличьте время стабилизации (`spec.behavior.scaleUp.stabilizationWindowSeconds`), чтобы избежать резкого увеличения количества подов.

## Ограничения HPA

1. По умолчанию HPA по-разному обрабатывает масштабирование вверх и вниз:

   - Масштабирование вверх:
     - Происходит незамедлительно (`spec.behavior.scaleUp.stabilizationWindowSeconds = 0`).
     - Ограничение — за 15 секунд число подов может удвоиться. Если подов было меньше 4, то добавятся 4 новых пода.

   - Масштабирование вниз:
     - Происходит в течение 5 минут (`spec.behavior.scaleUp.stabilizationWindowSeconds = 300`).
     - Собираются «предложения» о новом количестве реплик, и выбирается самое большое, чтобы не было слишком частого уменьшения.
     - Нет ограничений на количество убираемых подов.

1. Масштабирование только одного контроллера. Нельзя настроить несколько HPA, нацеленных на один и тот же Deployment (или StatefulSet), иначе они будут конфликтовать.

## Как включить или отключить HPA

HPA не требует отдельного включения в DKP. Но, если необходимо маштабирование не только по метрикам по потреблению CPU и памяти подов, то необходимо включить модуль [`prometheus-metrics-adapter`](/modules/prometheus-metrics-adapter/). Как включить модуль [см. в документации](scaling-by-metrics.html#как-включить-prometheus-metrics-adapter).

## Настройка HPA

Для настройки HPA необходимо:

1. Определить контроллер (Deployment или StatefulSet), который будет масштабироваться.

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

1. При необходимости, в параметре `stabilizationWindowSeconds` контроллера настроить время, в течение которого масштабирование может быть отменено, чтобы ограничить скорость прироста подов:

   ```yaml
   behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
    # Позволяет отложить принятие решения о масштабировании вверх и ограничить скорость прироста подов.
   ```

## Примеры настройки HPA

### Масштабирование по CPU и памяти

Масштабирование происходит, если среднее использование CPU/памяти всех подов превышает указанный процент:

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

### Масштабирование по метрикам объектов

Если метрика `rmq-queue-forum-messages` (количество сообщений в RabbitMQ) превышает 42, HPA увеличивает число реплик Deployment myconsumer:

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

### Масштабирование на основе внешних данных

Используется метрика `mymetric`, получаемая из внешнего источника (например, SQS). Масштабирование происходит, если значение превышает 100:

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
