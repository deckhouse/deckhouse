---
title: "Маршрутизация запросов с Istio"
permalink: ru/user/network/request_routing_istio.html
lang: ru
---

Для маршрутизации HTTP- и TCP-запросов можно использовать [istio](../reference/mc/istio/).
Перед настройкой маршрутизации убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%BC%D0%B0%D1%80%D1%88%D1%80%D1%83%D1%82%D0%B8%D0%B7%D0%B0%D1%86%D0%B8%D1%8F-%D0%B7%D0%B0%D0%BF%D1%80%D0%BE%D1%81%D0%BE%D0%B2 -->

Основной ресурс для управления маршрутизацией — [VirtualService](#ресурс-virtualservice) от istio.io, он позволяет переопределять судьбу HTTP- или TCP-запроса. Доступные аргументы для принятия решения о маршрутизации:

* Host и любые другие заголовки;
* URI;
* метод (GET, POST и пр.);
* лейблы пода или namespace источника запросов;
* dst-IP или dst-порт для не-HTTP-запросов.

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
