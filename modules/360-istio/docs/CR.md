---
title: "Модуль istio: Custom Resources"
---

## Маршрутизация

### DestinationRule

[Reference](https://istio.io/latest/docs/reference/config/networking/destination-rule/).

Настройка исходящих запросов на сервис:
* балансировка трафика между эндпоинтами,
* лимиты TCP-соединений и реквестов,
* Sticky Sessions,
* Circuit Breaker,
* определение версий сервиса для Canary Deployment,
* настройка tls для исходящих запросов.

### VirtualService

[Reference](https://istio.io/latest/docs/reference/config/networking/virtual-service/).

Использование VirtualService опционально, классические сервисы продолжают работать если вам достаточно их функционала.

Гибкая настройка маршрутизации и распределения нагрузки между классическими сервисами и DestinationRule-ами на основе веса, заголовков, лейблов, uri и пр. Тут можно использовать subset-ы ресурса [DestinationRule](#destinationrule).


> **Важно!** Istio должен знать о существовании `destination`, если вы используете внешний API, то зарегистрируйте его через [ServiceEntry](#serviceentry).

### ServiceEntry

[Reference](https://istio.io/latest/docs/reference/config/networking/service-entry/).

Аналог Endpoints + Service из ванильного Kubernetes. Позволяет сообщить Istio о существовании внешнего сервиса или даже переопределить его адрес.

## Аутентификация

Решает задачу "кто сделал запрос?". Не путать с авторизацией, которая определяет, "разрешить ли аутентифицированному элементу делать что-то или нет?".

### Policy

Reference (Не актуальная ссылка - `https://istio.io/docs/reference/config/istio.authentication.v1alpha1/#Policy`).

Локальные настройки аутентификации на стороне приёмника (сервиса). Можно определить JWT-аутентификацию или включить/выключить mTLS для какого-то сервиса.
Для глобального включения mTLS используйте [параметр](/modules/360-istio/configuration.html#параметры) `tlsMode`.

## Авторизация

Есть два метода авторизации:
* Native — средствами `istio-proxy`, не требует Mixer, позволяет настроить правила вида "сервис А имеет доступ к сервису Б".
* Mixer — позволяет настраивать более сложные правила, включая квоты RPS, whitelisting, кастомные методы и пр. В данном модуле **не реализована** поддержка авторизации средствами Mixer.

### Native-авторизация

**Важно!** Авторизация без mTLS-аутентификации не будет работать в полной мере. В этом случае будут доступны только простейшие аргументы для составления политик, такие как source.ip и request.headers.

#### RbacConfig

Reference (Не актуальная ссылка - `https://istio.io/docs/reference/config/authorization/istio.rbac.v1alpha1/#RbacConfig-Mode`).

ВКЛ/ВЫКЛ нативную авторизацию для namespace или для отдельных сервисов. Если авторизация включена — работает правило "всё, что не разрешено — запрещено".

#### ServiceRole

Reference (Не актуальная ссылка - https://istio.io/docs/reference/config/authorization/istio.rbac.v1alpha1/#ServiceRole`).

Определяет **ЧТО** разрешено.

#### ServiceRoleBinding

> Устаревшая ссылка на Reference - `https://istio.io/docs/reference/config/authorization/istio.rbac.v1alpha1/#ServiceRoleBinding`.

Привязывает [**ЧТО**](#servicerole) разрешено **КОМУ** (spec.subjects).

При этом **КОГО** можно определить несколькими способами:

* ServiceAccount — указать в поле users sa пода, из которого обращаются.
* На основе аргументов из запроса, включая данные из JWT-токена. Полный список на [официальном сайте](https://istio.io/latest/docs/reference/config/security/conditions/).


```yaml
apiVersion: "rbac.istio.io/v1alpha1"
kind: ServiceRoleBinding
metadata:
  name: binding-apis
  namespace: myns
spec:
  subjects:
  - user: "cluster.local/ns/myns/sa/my-service-account"
  - properties:
      request.headers[X-Secret-Header]: "la-resistance"
  roleRef:
    kind: ServiceRole
    name: "api-user
```

