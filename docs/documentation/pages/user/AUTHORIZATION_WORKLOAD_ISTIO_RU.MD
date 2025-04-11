---
title: "Управление авторизацией и доступом к workload c Istio"
permalink: ru/user/authorization-workload-istio.html
lang: ru
---

Для управления авторизацией и контролем доступа к workload можно использовать [istio](../reference/mc/istio/).
Перед настройкой авторизации убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%B0%D0%B2%D1%82%D0%BE%D1%80%D0%B8%D0%B7%D0%B0%D1%86%D0%B8%D1%8F -->

## Авторизация

Управление авторизацией осуществляется с помощью ресурса [AuthorizationPolicy](#ресурс-authorizationpolicy) от istio.io. В момент, когда для сервиса создается этот ресурс, начинает работать следующий алгоритм принятия решения о судьбе запроса:

* Если запрос попадает под политику DENY — запретить запрос.
* Если для данного сервиса нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если явно что-то запретить, работает только запрет. Если же что-то явно разрешить, будут разрешены только явно одобренные запросы (запреты при этом имеют приоритет).

Для написания правил авторизации можно использовать следующие аргументы:

* идентификаторы сервисов и wildcard на их основе (`mycluster.local/ns/myns/sa/myapp` или `mycluster.local/*`);
* namespace;
* диапазоны IP;
* HTTP-заголовки;
* JWT-токены из прикладных запросов.

## Ресурс AuthorizationPolicy

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/istio-cr.html#authorizationpolicy -->

[Reference](https://istio.io/v1.19/docs/reference/config/security/authorization-policy/).

Включает и определяет контроль доступа к workload. Поддерживает как ALLOW-, так и DENY-правила. Как только у workload появляется хотя бы одна политика, начинает работать следующий приоритет:

* Если запрос попадает под политику DENY — запретить запрос.
* Если для данного приложения нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Аргументы для принятия решения об авторизации:
* source:
  * namespace;
  * principal (читай — идентификатор юзера, полученный после аутентификации);
  * IP.
* destination:
  * метод (GET, POST...);
  * Host;
  * порт;
  * URI.
* [conditions](https://istio.io/v1.19/docs/reference/config/security/conditions/#supported-conditions):
  * HTTP-заголовки
  * аргументы source
  * аргументы destination
  * JWT-токены
