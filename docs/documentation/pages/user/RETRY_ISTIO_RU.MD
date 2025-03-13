---
title: "Настройка Retry для запросов с Istio"
permalink: ru/user/retry_istio.html
lang: ru
---

Для Retry для запросов можно использовать [istio](#).
Перед настройкой убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#retry -->

Чтобы можно настроить Retry для запросов, используйте ресурс [VirtualService](#).

**Внимание!** По умолчанию при возникновении ошибок все запросы (включая POST-запросы) выполняются повторно до трех раз.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: ratings-route
spec:
  hosts:
  - ratings.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: ratings.prod.svc.cluster.local
    retries:
      attempts: 3
      perTryTimeout: 2s
      retryOn: gateway-error,connect-failure,refused-stream
```
