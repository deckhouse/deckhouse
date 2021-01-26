---
title: "Модуль istio: примеры конфигурации"
---

## Включить балансировку для сервиса `ratings.prod.svc.cluster.local`

Был обыкновенный сервис `myservice`, который балансился через iptables, а мы включили умную балансировку.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-lb
  namespace: prod
spec:
  host: myservice.prod.svc.cluster.local
  trafficPolicy:
    loadBalancer:
      simple: LEAST_CONN
```

## Добавить к сервису myservice.prod.svc дополнительные, вторичные subset-ы со своими правилами

Эти subset-ы работают при использовании [VirtualService](cr.html#virtualservice):
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-extra-subsets
spec:
  host: myservice.prod.svc.cluster.local
  trafficPolicy: # срабатывает если определён лишь классический Service.
    loadBalancer:
      simple: LEAST_CONN
  subsets: # subset-ы необходимо определить через VirtualService, где эти subset-ы указаны в маршрутах.
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

## Circuit Breaker

Для единственного сервиса потребуется единственный CR [DestinationRule](cr.html#destinationrule).

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-circuit-breaker
spec:
  host: myservice.prod.svc.cluster.local # либо полный FQDN, либо локальный для namespace домен.
  trafficPolicy:
    outlierDetection:
      consecutiveErrors: 7 # можно допустить не более семи ошибок
      interval: 5m # в течение пяти минут,
      baseEjectionTime: 15m # при этом проблемный эндпоинт будет исключён из работы на 15 минут.
```

## Retry

Для единственного сервиса потребуется единственный CR [VirtualService](cr.html#virtualservice).

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: productpage-retry
spec:
  hosts:
    - productpage # либо полный FQDN, либо локальный для namespace домен.
  http:
  - route:
    - destination:
        host: productpage # Хотя бы один destination или rewrite необходим. В данном примере не меняем направление.
    timeout: 8s
    retries:
      attempts: 3
      perTryTimeout: 3s
```

## Canary

Подразумевается, что в одном namespace выкачено два Deployment с разными версиями приложения. У подов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два CR:
* [DestinationRule](cr.html#destinationrule) с описанием, как идентифицировать разные версии вашего приложения.
* [VirtualService](cr.html#virtualservice) с описанием, как распределять трафик между разными версиями приложения.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  subsets: # subset-ы работают только если к хосту обращаются через VirtualService, в котором эти subset-ы указаны в маршрутах.
  - name: v1
    labels: # аналог selector у Service. Поды с такими лейблами попадут под действие этого subset-a.
      version: v1
  - name: v2
    labels:
      version: v2
```
```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - route:
    - destination:
        host: productpage
        subset: v1 # ссылка на subset из DestinationRule
      weight: 90 # процент трафика, который получат поды с лейблом version: v1.
  - route:
    - destination:
        host: productpage
        subset: v2
      weight: 10
```


### Распределение нагрузки между сервисами с разными версиями для Canary Deployment
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

## Ingress

Для работы с Ingress требуется подготовить:
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` у CR IngressNginxController модуля [ingress-nginx](/modules/402-ingress-nginx). Данный контроллер сможет обслуживать толко Istio-окружение!
* Ingress, который ссылается на Service. Обязательные аннотации для Ingress:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — таким образом ingress-контроллер будет отправлять запросы на единственный ClusterIP, работу по балансировке будет делать envoy.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc.cluster.local` — Istio не парсит host из Ingress, с данной аннотацией envoy сможет идентифицировать прикладной сервис. Другой подход — создавать `VirtualService` с публичным FQDN.

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    nginx.ingress.kubernetes.io/service-upstream: "true" # Просим nginx проксировать трафик на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc.cluster.local # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов. Чтобы сообщать Istio о существовании внешнего домена `productpage.example.com`, мы просто используем внутренний домен, о котором Istio осведомлён.
spec:
  rules:
    - host: productpage.example.com
      http:
        paths:
        - path: /
          backend:
            serviceName: productpage
            servicePort: 9080
```
```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

## Примеры настройки авторизации

### Алгоритм принятия решения

**Важно!** Как только для приложения создаётся AuthorizationPolicy, начинает работать следующий алгоритм принятия решения о судьбе запроса:
* Если под запрос есть правило DENY — запретить.
* Если для данного приложения нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если вы явно что-то запретили, то работает только ваш запрет. Если же вы что-то явно разрешили, то теперь разрешены только явно одобренные запросы (запреты никуда не деваются и имеют приоритет).

Примеры:
* Запретим POST-запросы для приложения myapp. Отныне, так как для приложения появилась политика, то согласно алгоритму выше будут запрещены только POST-запросы к приложению.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
	name: deny-post-requests
	namespace: foo
  spec:
	selctor:
	  app: myapp
	action: DENY
	rules:
	- to:
	  - operation:
		  methods: ["POST"]
  ```

* Здесь для приложения создана полтитика ALLOW. При ней будут разрешены только запросы из NS `bar`. Остальные — запрещены.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
	name: deny-all
	namespace: foo
  spec:
	selctor:
	  app: myapp
	action: ALLOW # default, можно не указывать
	rules:
	- from:
	  - source:
		  namespaces: ["bar"]
  ```

* Здесь для приложения создана полтитика ALLOW. При этом она не имеет ни одного правила и поэтому ни один запрос под неё не попадёт, но она таки есть. Поэтому, согласно алгоритму, раз что-то разрешено, то всё остальное — запрещено. В данном случае всё остальное — это вообще все запросы.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
	name: deny-all
	namespace: foo
  spec:
	selctor:
	  app: myapp
	action: ALLOW # default, можно не указывать
	rules: []
  ```

* Здесь для приложения создана политика ALLOW (это default) и одно пустое правило. Под это правило попадает любой запрос и автоматически этот запрос получает добро.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
	name: allow-all
	namespace: foo
  spec:
	selctor:
	  app: myapp
	rules:
	- {}
  ```

### Запретить вообще всё в рамках NS foo

Два способа:

* Запретить явно. Здесь мы создаём политику DENY с единственным универсальным фильтром `{}`, под который попадают все запросы:

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
	name: deny-all
	namespace: foo
  spec:
	action: DENY
	rules:
	- {}
  ```

* Неявно. Здесь мы создаём политику ALLOW (по умолчанию), но не создаём ни одного фильтра так, что ни один запрос под неё не попадёт и будет автоматически запрещён.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
	name: deny-all
	namespace: foo
  spec: {}
  ```

### Запретить доступ только из NS foo

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: deny-from-ns-foo
 namespace: myns
spec:
 action: DENY
 rules:
 - from:
   - source:
       namespaces: ["foo"]
```

### Разрешить запросы только в рамках нашего NS foo

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-intra-namespace-only
 namespace: foo
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       namespaces: ["foo"]
```

### Разрешить из любого места в нашем кластере

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-my-cluster
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["mycluster.local/*"]
```

### Разрешить любые запросы только кластеров foo или bar

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-foo-or-bar-clusters-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["foo.local/*", "bar.local/*"]
```

### Разрешить любые запросы только кластеров foo или bar, при этом из NS baz

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-foo-or-bar-clusters-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source: # правила ниже логически перемножаются
       namespaces: ["baz"]
       principals: ["foo.local/*", "bar.local/*"]
```

### Разрешить из любого кластера (по mtls)

**Важно!** Если есть запрещающие правила, то у них будет приоритет. См. [алгоритм](#алгоритм-принятия-решения).

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any-cluster-with-mtls
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["*"] # to set MTLS mandatory
```

### Разрешить вообще откуда угодно (в том числе без mtls)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any
 namespace: myns
spec:
 action: ALLOW
 rules: [{}]
```
