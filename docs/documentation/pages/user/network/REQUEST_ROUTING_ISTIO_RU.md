---
title: "Маршрутизация запросов с Istio"
permalink: ru/user/network/request_routing_istio.html
lang: ru
---

Для маршрутизации HTTP- и TCP-запросов в Deckhouse Kubernetes Platform можно использовать модуль [`istio`](/modules/istio/).

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%BC%D0%B0%D1%80%D1%88%D1%80%D1%83%D1%82%D0%B8%D0%B7%D0%B0%D1%86%D0%B8%D1%8F-%D0%B7%D0%B0%D0%BF%D1%80%D0%BE%D1%81%D0%BE%D0%B2 -->

Основной ресурс для управления маршрутизацией — [VirtualService](#ресурс-virtualservice) от Istio, он позволяет настраивать маршрутизацию HTTP- или TCP-запросов.

## Ресурс VirtualService

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/istio-cr.html#virtualservice -->

Подробнее ознакомиться с VirtualService можно в документации [istio](https://istio.io/v1.19/docs/reference/config/networking/virtual-service/).

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
Для корректной работы `destination`в Istio необходимо его указать. Если вы используете внешний API, укажите его с помощью [ServiceEntry](/modules/istio/istio-cr.html#serviceentry).
{% endalert %}
