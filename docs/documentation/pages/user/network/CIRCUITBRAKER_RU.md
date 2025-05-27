---
title: "Circuit Breaker"
permalink: ru/user/network/circuit-breaker.html
lang: ru
---

В Deckhouse Kubernetes Platform Circuit Breaker реализуется средствами Istio (модуль [`istio`](../reference/mc/istio/)) и обеспечивает следующие возможности:

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%B7%D0%B0%D0%B4%D0%B0%D1%87%D0%B8-%D0%BA%D0%BE%D1%82%D0%BE%D1%80%D1%8B%D0%B5-%D1%80%D0%B5%D1%88%D0%B0%D0%B5%D1%82-istio -->

* временное исключение эндпоинта из балансировки, если превышен лимит ошибок;
* настройка лимитов на количество TCP-соединений и количество запросов в сторону одного эндпоинта;
* выявление зависших запросов и обрывание их с кодом ошибки (HTTP request timeout).

## Пример настройки Circuit Breaker

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#circuit-breaker -->

Для выявления проблемных эндпоинтов используются настройки `outlierDetection` в custom resource [DestinationRule](../user/managing_request_between_service_istio.html#ресурс-destinationrule).
Более подробно алгоритм Outlier Detection описан в [документации Envoy](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier).

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: reviews-cb-policy
spec:
  host: reviews.prod.svc.cluster.local
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 100 # Максимальное число коннектов в сторону host, суммарно для всех эндпоинтов.
      http:
        maxRequestsPerConnection: 10 # Каждые 10 запросов коннект будет пересоздаваться.
    outlierDetection:
      consecutive5xxErrors: 7 # Допустимо 7 ошибок (включая пятисотые, TCP-таймауты и HTTP-таймауты)
      interval: 5m            # в течение пяти минут,
      baseEjectionTime: 15m   # после которых эндпоинт будет исключен из балансировки на 15 минут.
```

А также для настройки HTTP-таймаутов используется ресурс [VirtualService](../user/retry_istio.html#ресурс-virtualservice). Эти таймауты также учитываются при подсчете статистики ошибок на эндпоинтах.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: my-productpage-rule
  namespace: myns
spec:
  hosts:
  - productpage
  http:
  - timeout: 5s
    route:
    - destination:
        host: productpage
```
