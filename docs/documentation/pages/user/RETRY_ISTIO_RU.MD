---
title: "Настройка Retry для запросов с Istio"
permalink: ru/user/retry_istio.html
lang: ru
---

Для Retry для запросов можно использовать [istio](../reference/mc/istio/).
Перед настройкой убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#retry -->

Чтобы можно настроить Retry для запросов, используйте ресурс [VirtualService](#ресурс-virtualservice) от istio.io.

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

## Ресурс VirtualService

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/istio-cr.html#virtualservice -->

[Reference](https://istio.io/v1.19/docs/reference/config/networking/virtual-service/)

Использование VirtualService опционально, классические сервисы продолжают работать, если вам достаточно их функционала.

Позволяет настроить маршрутизацию запросов:
* Аргументы для принятия решения о маршруте:
  * Host;
  * URI;
  * вес.
* Параметры итоговых направлений:
  * новый хост;
  * новый URI;
  * если хост определен с помощью [DestinationRule](#destinationrule), можно направлять запросы на subset'ы;
  * таймаут и настройки ретраев.

> **Важно!** Istio должен знать о существовании `destination`, если вы используете внешний API, то зарегистрируйте его через [ServiceEntry](#serviceentry).
