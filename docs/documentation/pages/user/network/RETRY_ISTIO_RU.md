---
title: "Настройка Retry для запросов с Istio"
permalink: ru/user/network/retry_istio.html
lang: ru
---

Для повторных попыток (Retry) для запросов можно использовать модуль [`istio`](/modules/istio/).
Перед настройкой убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#retry -->

Чтобы настроить Retry для запросов, используйте ресурс [VirtualService](#ресурс-virtualservice) от istio.io.

{% alert level="warning" %}
По умолчанию, при возникновении ошибок, все запросы (включая POST-запросы) выполняются повторно до трех раз.
{% endalert %}

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

При необходимости ознакомьтесь с [документацией VirtualService](https://istio.io/v1.19/docs/reference/config/networking/virtual-service/).

Использование VirtualService опционально, классические сервисы продолжают работать, если их функционала достаточно. С помощью этого ресурса можно настроить маршрутизацию запросов:

* Аргументы для принятия решения о маршруте:
  * `host`;
  * `uri`;
  * `weight` (вес).
* Параметры итоговых направлений:
  * новый `host`;
  * новый `uri`;
  * если `host` определен с помощью [DestinationRule](../network/managing_request_between_service_istio.html#ресурс-destinationrule) можно направлять запросы на subset'ы;
  * таймаут и настройки retry (повторных попыток).

{% alert level="warning" %}
Для корректной работы `destination` в Istio необходимо его указать. Если вы используете внешний API, укажите его с помощью [ServiceEntry](/modules/istio/istio-cr.html#serviceentry).
{% endalert %}
