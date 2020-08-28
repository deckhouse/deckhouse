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

#### Примеры

##### Включить балансировку для сервиса `ratings.prod.svc.cluster.local`:
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: bookinfo-ratings
spec:
  host: ratings.prod.svc.cluster.local
  trafficPolicy:
    loadBalancer:
      simple: LEAST_CONN
```

##### Добавить к основному DestinationRule дополнительные, вторичные subset-ы со своими правилами.

Эти subset-ы работают при использовании [VirtialService](#virtualservice):
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: bookinfo-ratings
spec:
  host: ratings.prod.svc.cluster.local
  trafficPolicy: # срабатывает если к хосту обратились через классический Service.
    loadBalancer:
      simple: LEAST_CONN
  subsets: # subset-ы работают только если к хосту обращаются через VirtualService, в котором эти subset-ы указаны в маршрутах.
  - name: testv1
    labels: # аналог selector у Service. Поды с такими лейблами попадут под действие этого subset-a.
      version: v1
  - name: testv3
    labels:
      version: v3
    trafficPolicy:
      loadBalancer:
        simple: ROUND_ROBIN
```

Ещё примеры:
* [Circuit Breaker](usage.html#circuit-breaker)
* [Canary](usage.html#canary)

### VirtualService

[Reference](https://istio.io/latest/docs/reference/config/networking/virtual-service/).

Использование VirtualService опционально, классические сервисы продолжают работать если вам достаточно их функционала.

Гибкая настройка маршрутизации и распределения нагрузки между классическими сервисами и DestinationRule-ами на основе веса, заголовков, лейблов, uri и пр. Тут можно использовать subset-ы ресурса [DestinationRule](#destinationrule).

#### Примеры
##### Распределение нагрузки между сервисами с разными версиями для Canary Deployment
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: reviews-route
spec:
  hosts:
  - reviews.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: reviews.prod.svc.cluster.local
        subset: testv1 # ссылка на subset из DestinationRule
      weight: 25
  - route:
    - destination:
        host: reviews.prod.svc.cluster.local
        subset: testv3
      weight: 75
```

##### Перенаправление location /uploads в другой сервис
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: uploads-route
spec:
  hosts:
  - gallery.prod.svc.cluster.local
  http:
  - match:
    - uri:
        prefix: "/uploads" # если обратились на gallery.prod.svc.cluster.local/uploads/a.jpg
    rewrite:
      uri: "/data" # то меняем uri на /data/a.jpg
    route:
    - destination:
        host: share.prod.svc.cluster.local # и обращаемся к share.prod.svc.cluster.local/data/a.jpg
  - route:
    - destination:
        host: gallery.prod.svc.cluster.local # остальные запросы оставляем как есть
```

Ещё примеры:
* [Circuit Breaker](usage.html#circuit-breaker)
* [Canary](usage.html#canary)

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

```yaml
apiVersion: authentication.istio.io/v1alpha1
kind: Policy
metadata:
  name: productpage-mTLS-with-JWT
  namespace: frod
spec:
  targets:
  - name: productpage # включить данную политику аутентификации для единственного сервиса "productpage"
    ports:
    - number: 9000
  peers: # Как аутентифицировать узел, с которого пришёл запрос
  - mtls: # Все запросы от узлов должны проходить через TLS-шифрование
      mode: STRICT
  origins: # Узлом могут воспользоваться разные конечные пользователи и мы можем их отличать с помощью их JWT-токенов.
  - jwt:
      issuer: "https://securetoken.google.com"
      audiences:
      - "productpage"
      jwksUri: "https://www.googleapis.com/oauth2/v1/certs"
      jwt_headers:
      - "x-goog-iap-jwt-assertion"
      trigger_rules: # Не требовать JWT-аутентификацию для локейшна /health_check
      - excluded_paths:
        - exact: /health_check
  principalBinding: USE_ORIGIN # Чьё авторство присваивать запросу? Узел или пользователь узла? В нашем случае — пользователь.
```

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

```yaml
apiVersion: "rbac.istio.io/v1alpha1"
kind: ServiceRole
metadata:
  name: api-user
  namespace: myns
spec:
  rules:
  - services: ["store.prod.svc.cluster.local"]
    methods: ["POST"]
    paths: ["/rest"]
  - services: ["api.prod.svc.cluster.local"]
    methods: ["GET"]
```

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

