---
title: "Модуль istio: Custom Resources (от istio.io)"
---

## Маршрутизация

### DestinationRule

[Reference](https://istio.io/latest/docs/reference/config/networking/destination-rule/).

Позволяет:
* Определить стратегию балансировки трафика между эндпоинтами сервиса:
  * Алгоритм балансировки (LEAST_CONN, ROUND_ROBIN, ...)
  * Признаки смерти эндпоинта и правила его выведения из балансировки
  * Лимиты TCP-соединений и реквестов для эндпоинтов
  * Sticky Sessions
  * Circuit Breaker
* Определить альтернативные группы эндпоинтов для обработки трафика (применимо для Canary Deployments). При этом, у каждой группы можно настроить свои стратегии балансировки.
* Настройка tls для исходящих запросов.

### VirtualService

[Reference](https://istio.io/latest/docs/reference/config/networking/virtual-service/).

Использование VirtualService опционально, классические сервисы продолжают работать если вам достаточно их функционала.

Позволяет настроить маршрутизацию запросов:
* Аргументы для принятия решения о маршруте:
  * Host
  * uri
  * Вес
* Параметры итоговых направлений:
  * Новый хост
  * Новый uri
  * Если хост определён с помощью [DestinationRule](#destinationrule), то можно направлять запросы на subset'ы
  * Таймаут и настройки ретраев

> **Важно!** Istio должен знать о существовании `destination`, если вы используете внешний API, то зарегистрируйте его через [ServiceEntry](#serviceentry).

### ServiceEntry

[Reference](https://istio.io/latest/docs/reference/config/networking/service-entry/).

Аналог Endpoints + Service из ванильного Kubernetes. Позволяет сообщить Istio о существовании внешнего сервиса или даже переопределить его адрес.

## Аутентификация

Решает задачу "кто сделал запрос?". Не путать с авторизацией, которая определяет, "разрешить ли аутентифицированному элементу делать что-то или нет?".

По факту есть два метода аутентификации:
* mTLS
* JWT-токены

### PeerAuthentication

[Reference](https://istio.io/latest/docs/reference/config/security/peer_authentication/).

Позволяет определить стратегию MTLS в отдельном NS. Принимать или нет нешифрованные запросы. Каждый mTLS-запрос автоматически позволяет определить источник и использовать его в правилах авторизации.

### RequestAuthentication

[Reference](https://istio.io/latest/docs/reference/config/security/request_authentication/).

Позволяет настроить JWT-аутентификацию для реквестов.

## Авторизация

**Важно!** Авторизация без mTLS- или jwt-аутентификации не будет работать в полной мере. В этом случае будут доступны только простейшие аргументы для составления политик, такие как source.ip и request.headers.

### AuthorizationPolicy

[Reference](https://istio.io/latest/docs/reference/config/security/authorization-policy/).

Включает и определяет контроль доступа к workload. Поддерживает как ALLOW, так и DENY правила. Как только у workload появляется хотя бы одна политика, то начинает работать приоритет:

* Если под реквест есть политика DENY, то отклонить запрос
* Если под реквест нет политики ALLOW, то разрешить запрос
* Если под реквест есть политика ALLOW, то разрешить запрос
* Отклонить запрос

Аргументы для принятия решения об авторизации:
* source:
  * namespace
  * principal (читай идентификатор юзера, полученный после аутентификации)
  * ip
* destination:
  * метод (GET, POST, ...)
  * Host
  * порт
  * uri

### Sidecar

[Reference](https://istio.io/latest/docs/reference/config/networking/sidecar/)

Данный ресурс позволяет ограничить количество сервисов, о которых будет передана информация в сайдкар istio-proxy.
