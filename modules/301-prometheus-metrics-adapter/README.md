Модуль prometheus-metrics-adapter
==========================

## Назначение

**TLDR;** — модуль позволяет работать автоскейлеру в кластере через [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) и [VPA](https://github.com/deckhouse/deckhouse/blob/master/modules/302-vertical-pod-autoscaler/README.md) по «любым» метрикам.

Данный модуль устанавливает в кластер [имплементацию](https://github.com/DirectXMan12/k8s-prometheus-adapter) Kubernetes [resource metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md) и [custom metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/custom-metrics-api.md) для получения метрик из Prometheus.

Это позволяет:
- kubectl top брать метрики из Prometheus, через адаптер, а не из heapster;
- использовать [autoscaling/v2beta1](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#horizontalpodautoscaler-v2beta1-autoscaling) для скейлинга приложений (HPA);
- получать информацию о потреблении ресурсов подами из prometheus средствами api kubernetes для других модулей (Vertical Pod Autoscaler, ...).

В будуших релизах, после принятия [данного MR'а](https://github.com/DirectXMan12/k8s-prometheus-adapter/pull/146) мы сможем скейлить приложения по абсолютно любым метрикам из Prometheus, так как данный MR добавляет имплементацию [external.metrics.k8s.io](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/external-metrics-api.md)

Сейчас данный модуль позволяет производить скейлинг по таким параметрам:
* cpu (pod'а)
* memory (pod'а)
* rps (ingress'а) - за 1,5,15 минут (`rps_Nm`);
* cpu (pod'а) - за 1,5,15 минут (`cpu_Nm`) - среднее потребления CPU за N минут;
* memory (pod'a) - за 1,5,15 минут (`memory_Nm`) - среднее потребление Memory за N минут.

Описание метрик, по которым можно производить скейлинг находится в [configmap](templates/config-map.yaml). Описать дополнительные метрики для скейлинга можно с помощью [документации](https://github.com/DirectXMan12/k8s-prometheus-adapter/blob/v0.4.1/docs/walkthrough.md).

##  Конфигурация

В общем случае не требует конфигурации. Требует конфигурации, если вы хотите скейлить не только по cpu и ram, но и по любым метрикам `prometheus`. Как пример — скейлинг воркеров по очереди в rabbitmq: docs/kb#349.

По умолчанию — **включен** в кластерах начиная с версии 1.9, если включен модуль `prometheus`.

### Параметры

* `highAvailability` — ручное управление [режимом отказоустойчивости](/FEATURES.md#отказоустойчивость).

## Как работает

Данный модуль регистрирует k8s-prometheus-adapter, как external API сервис, который расширяет возможности Kubernetes API сервера с помощью сторонних приложений (в данном случае k8s-prometheus-adapter). Когда какому-то из компонентов Kubernetes (VPA, HPA) необходима информация об используемых ресурсах, запрос уходит в Kubernetes API сервер, откуда запрос по TLS уходит в адаптер. Адаптер на основе своего [конфигурационного файла](templates/config-map.yaml) узнает, что нужно сделать для получения метрики, и отправляет запрос в Prometheus кластера.

## Пример использования Horizontal Pod Autoscaler

Пример HPA для скейлинга по всем доступным параметрам [API Reference](https://v1-10.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.10/#horizontalpodautoscaler-v2beta1-autoscaling):

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta1
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Object
    object:
      metricsName: rps_1m
      target:
        apiVersion: extensions/v1beta1
        kind: Ingress
        name: app
      targetValue: 1k
  - type: Pods
    pods:
      metricName: cpu_15m
      targetAverageValue: 500m
  - type: Resource
    resource:
      name: cpu
      targetAverageUtilization: 50
   - type: Resource
    resource:
      name: memory
      targetAverageValue: 10Mi
```

## Как добавить кастомные правила

А очень просто! Любое правило, добавленное в `cm/prometheus-metrics-adapter-custom`, автоматически попадает в конфигурационный файл `prometheus-metrics-adapter`.

* Для custom правил поддерживается только один cm (и пока это не является ограничением)
* Этот cm не создается автоматически (и не управляется antiop'ой), так что если его нет — его нужно просто создать: `kubectl -n kube-prometheus create cm prometheus-metrics-adapter-custom`.
* Любые изменения (в том числе и создание/удаление cm/prometheus-metrics-adapter-custom) подхватываются полностью автоматически, но требуется подождать около минуты (пока kubernetes зальет данные из cm в pod).
* Пример того, что нужно складывать в этот cm:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-metrics-adapter-custom
  namespace: kube-prometheus
data:
  rabbitmq-queue: |
    - seriesQuery: 'rabbitmq_queue_messages{job="rabbitmq",namespace!="",pod=~"rabbitmq-0",queue=~"stock_changes",service="rmq",vhost="/"}'
      resources:
        overrides:
          namespace: {resource: namespace}
      name:
        matches: ".*"
        as: "last_queue_depth_stock_changes"
      metricsQuery: 'sum (<<.Series>>{<<.LabelMatchers>>,queue=~"stock_changes"}) by (<<.GroupBy>>)'
```

 где `stock_changes` - имя очереди, `last_queue_depth_stock_changes` - имя метрики, в которую мы будем выдавать значение, полученное из prometheus.
