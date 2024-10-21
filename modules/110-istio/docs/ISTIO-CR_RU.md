---
title: "Модуль istio: Custom Resources (от istio.io)"
---

## Маршрутизация

### DestinationRule

[Reference](https://istio.io/v1.19/docs/reference/config/networking/destination-rule/)

Позволяет:
* Определить стратегию балансировки трафика между эндпоинтами сервиса:
  * алгоритм балансировки (LEAST_CONN, ROUND_ROBIN, ...);
  * признаки смерти эндпоинта и правила его выведения из балансировки;
  * лимиты TCP-соединений и реквестов для эндпоинтов;
  * Sticky Sessions;
  * Circuit Breaker.
* Определить альтернативные группы эндпоинтов для обработки трафика (применимо для Canary Deployments). При этом у каждой группы можно настроить свои стратегии балансировки.
* Настройка TLS для исходящих запросов.

### VirtualService

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

### ServiceEntry

[Reference](https://istio.io/v1.19/docs/reference/config/networking/service-entry/)

Аналог Endpoints + Service из ванильного Kubernetes. Позволяет сообщить Istio о существовании внешнего сервиса или даже переопределить его адрес.

## Аутентификация

Решает задачу «Кто сделал запрос?». Не путать с авторизацией, которая определяет, «разрешить ли аутентифицированному элементу делать что-то или нет».

По факту есть два метода аутентификации:
* mTLS;
* JWT-токены.

### PeerAuthentication

[Reference](https://istio.io/v1.19/docs/reference/config/security/peer_authentication/)

Позволяет определить стратегию mTLS в отдельном NS — принимать или нет нешифрованные запросы. Каждый mTLS-запрос автоматически позволяет определить источник и использовать его в правилах авторизации.

### RequestAuthentication

[Reference](https://istio.io/v1.19/docs/reference/config/security/request_authentication/)

Позволяет настроить JWT-аутентификацию для реквестов.

## Авторизация

**Важно!** Авторизация без mTLS- или JWT-аутентификации не будет работать в полной мере. В этом случае будут доступны только простейшие аргументы для составления политик, такие как `source.ip` и `request.headers`.

### AuthorizationPolicy

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

### Sidecar

[Reference](https://istio.io/v1.19/docs/reference/config/networking/sidecar/)

Данный ресурс позволяет ограничить количество сервисов, информация о которых будет передана в сайдкар istio-proxy.
