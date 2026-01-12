---
title: "Управление авторизацией и доступом к workload c Istio"
permalink: ru/user/network/authorization-workload-istio.html
lang: ru
---

Для управления авторизацией и контролем доступа к workload можно использовать модуль [istio](/modules/istio/).
Перед настройкой авторизации убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%B0%D0%B2%D1%82%D0%BE%D1%80%D0%B8%D0%B7%D0%B0%D1%86%D0%B8%D1%8F -->

## Авторизация

Управление авторизацией осуществляется с помощью ресурса [AuthorizationPolicy](#ресурс-authorizationpolicy) от Istio. Когда для сервиса создается этот ресурс, применяются следующие правила принятия решения о запросах:

* Если запрос попадает под политику `DENY` — запретить запрос.
* Если для данного сервиса нет политик `ALLOW` — разрешить запрос.
* Если запрос попадает под политику `ALLOW` — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если явно что-то запретить, работает только запрет. Если же что-то явно разрешить, будут разрешены только явно одобренные запросы (запреты при этом имеют приоритет).

Для написания правил авторизации можно использовать следующие аргументы:

* идентификаторы сервисов и wildcard на их основе (`mycluster.local/ns/myns/sa/myapp` или `mycluster.local/*`);
* пространство имен;
* диапазоны IP;
* HTTP-заголовки;
* JWT-токены из прикладных запросов.

## Ресурс AuthorizationPolicy

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/istio-cr.html#authorizationpolicy -->

Подробнее ознакомиться с AuthorizationPolicy можно [в документации Istio](https://istio.io/v1.19/docs/reference/config/security/authorization-policy/).

Ресурс AuthorizationPolicy включает и определяет контроль доступа к workload. Поддерживает как ALLOW-, так и DENY-правила, описанные выше.

Аргументы для принятия решения об авторизации:

* `source`:
  * `namespace`;
  * `principal` (идентификатор юзера, полученный после аутентификации);
  * IP.
* `destination`:
  * `method` (`GET`, `POST` и т. д.);
  * `host`;
  * `port`;
  * URI.
* [`conditions`](https://istio.io/v1.19/docs/reference/config/security/conditions/#supported-conditions):
  * HTTP-заголовки;
  * аргументы `source`;
  * аргументы `destination`;
  * JWT-токены.
