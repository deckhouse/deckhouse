---
title: "Модуль istio: примеры конфигурации"
---

## Примеры ресурсов

### IstioFederation

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: example-cluster
spec:
  metadataEndpoint: https://istio.k8s.example.com/metadata/
  trustDomain: example.local
```

### IstioMulticluster

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: example-cluster
spec:
  metadataEndpoint: https://istio.k8s.example.com/metadata/
```

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

Эти subset-ы работают при использовании [VirtualService](istio-cr.html#virtualservice):

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: myservice-extra-subsets
spec:
  host: myservice.prod.svc.cluster.local
  trafficPolicy: # Срабатывает, если определён лишь классический Service.
    loadBalancer:
      simple: LEAST_CONN
  subsets: # subset-ы необходимо определить через VirtualService, где эти subset-ы указаны в маршрутах.
  - name: testv1
    labels: # Аналог selector у Service. Pod'ы с такими лейблами попадут под действие этого subset.
      version: v1
  - name: testv3
    labels:
      version: v3
    trafficPolicy:
      loadBalancer:
        simple: ROUND_ROBIN
```

## Circuit Breaker

Для единственного сервиса потребуется единственный custom resource [DestinationRule](istio-cr.html#destinationrule).

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

Для единственного сервиса потребуется единственный custom resource[VirtualService](istio-cr.html#virtualservice).

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: productpage-retry
spec:
  hosts:
    - productpage # Либо полный FQDN, либо локальный для namespace домен.
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

Подразумевается, что в одном namespace выкачено два Deployment с разными версиями приложения. У Pod'ов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два custom resource:
* [DestinationRule](istio-cr.html#destinationrule) с описанием, как идентифицировать разные версии вашего приложения.
* [VirtualService](istio-cr.html#virtualservice) с описанием, как распределять трафик между разными версиями приложения.

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  subsets: # subset-ы работают только если к хосту обращаются через VirtualService, в котором эти subset-ы указаны в маршрутах.
  - name: v1
    labels: # Аналог selector у Service. Pod'ы с такими лейблами попадут под действие этого subset.
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
        subset: v1 # Ссылка на subset из DestinationRule.
      weight: 90 # Процент трафика, который получат Pod'ы с лейблом version: v1.
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
        subset: testv1 # Ссылка на subset из DestinationRule.
      weight: 25
    - destination:
        host: reviews.prod.svc.cluster.local
        subset: testv3
      weight: 75
```

#### Перенаправление location /uploads в другой сервис

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
        prefix: "/uploads" # Если обратились на gallery.prod.svc.cluster.local/uploads/a.jpg,
    rewrite:
      uri: "/data" # ... то меняем uri на /data/a.jpg,
    route:
    - destination:
        host: share.prod.svc.cluster.local # ... и обращаемся к share.prod.svc.cluster.local/data/a.jpg,
  - route:
    - destination:
        host: gallery.prod.svc.cluster.local # ...остальные запросы оставляем как есть.
```

## Ingress

Для работы с Ingress требуется подготовить:
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` у custom resource [IngressNginxController](../../modules/402-ingress-nginx/cr.html#ingressnginxcontroller) модуля [ingress-nginx](../../modules/402-ingress-nginx/).
* Ingress, который ссылается на Service. Обязательные аннотации для Ingress:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы слать их напрямую в Pod'ы приложения. Сайдкар `istio-proxy` перехватывает трафик только в сторону диапазона ServiceCIDR, остальные запросы отправляются напрямую.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — с данной аннотацией sidecar сможет идентифицировать прикладной сервис, для которого предназначен запрос.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    nginx.ingress.kubernetes.io/service-upstream: "true" # Просим nginx проксировать трафик на ClusterIP вместо собственных IP Pod'ов.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов. Чтобы не сообщать Istio о существовании внешнего домена `productpage.example.com`, мы просто используем внутренний домен, о котором Istio осведомлён.
spec:
  rules:
    - host: productpage.example.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: productpage
              port:
                number: 9080
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

**Важно!** Как только для приложения создаётся `AuthorizationPolicy`, начинает работать следующий алгоритм принятия решения о судьбе запроса:
* Если запрос попадает под политику DENY — запретить запрос.
* Если для данного приложения нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если вы явно что-то запретили, то работает только ваш запрет. Если же вы что-то явно разрешили, то теперь разрешены только явно одобренные запросы (запреты никуда не деваются и имеют приоритет).

**Важно!** Для работы политик, основанных на высокоуровневых параметрах, таких как namespace или principal, необходимо, чтобы все вовлечённые сервисы работали под управлением Istio. Также, между приложениями должен быть организован Mutual TLS, по умолчанию он организован, благодаря параметру модуля `tlsMode: MutualPermissive`.

Примеры:
* Запретим POST-запросы для приложения myapp. Отныне, так как для приложения появилась политика, то согласно алгоритму выше будут запрещены только POST-запросы к приложению.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-post-requests
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: DENY
    rules:
    - to:
      - operation:
          methods: ["POST"]
  ```

* Здесь для приложения создана политика ALLOW. При ней будут разрешены только запросы из NS `bar`. Остальные — запрещены.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: ALLOW # default, можно не указывать
    rules:
    - from:
      - source:
          namespaces: ["bar"]
  ```

* Здесь для приложения создана политика ALLOW. При этом она не имеет ни одного правила и поэтому ни один запрос под неё не попадёт, но она таки есть. Поэтому, согласно алгоритму, раз что-то разрешено, то всё остальное — запрещено. В данном случае всё остальное — это вообще все запросы.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: ALLOW # default, можно не указывать.
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
    selector:
      matchLabels:
        app: myapp
    rules:
    - {}
  ```

### Запретить вообще всё в рамках namespace foo

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

### Запретить доступ только из namespace foo

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

### Разрешить запросы только в рамках нашего namespace foo

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

### Разрешить любые запросы только кластеров foo или bar, при этом из namespace baz

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

## Управление поведением data-plane

### [экспериментальная функция] Предотвратить завершение работы istio-proxy до завершения соединений основного приложения

По умолчанию, в процессе остановки пода, все контейнеры, включая istio-proxy, получают сигнал SIGTERM одновременно. Но некоторым приложениям для правильного завершения работы необходимо время и иногда дополнительная сетевая активность. Это невозможно если istio-proxy завершился раньше.
Решение — добавить в istio-proxy preStop-хук для оценки состояния прикладных сокетов и если таковых нет, то можно останавливать контейнер.
Аннотация ниже добавляет описанный выше preStop-хук в контейнер istio-proxy прикладного пода:
`inject.istio.io/templates: sidecar,d8-hold-istio-proxy-termination-until-application-stops`.

## Обновление control-plane Istio

* Deckhouse позволяет инсталлировать несколько версий control-plane одновременно:
  * Одна глобальная, обслуживает namespace'ы или Pod'ы без явного указания версии (label у namespace `istio-injection: enabled`). Настраивается обязательным параметром `istio.globalVersion` в ConfigMap `deckhouse`.
  * Остальные — дополнительные, обслуживают namespace'ы или Pod'ы с явным указанием версии (label у namespace или у Pod `istio.io/rev: v1x13`). Настраиваются дополнительным параметром `istio.additionalVersions` в ConfigMap `deckhouse`.
* Istio заявляет обратную совместимость между data-plane и control-plane в диапазоне двух минорных версий:
![Istio data-plane and control-plane compatibility](https://istio.io/latest/blog/2021/extended-support/extended_support.png)
* Алгоритм обновления (для примера, на версию `1.13`):
  * Добавить желаемую версию в параметр модуля `istio.additionalVersions` в ConfigMap `deckhouse` (`additionalVersions: ["1.13"]`).
  * Дождаться появления соответствующего Pod'а `istiod-v1x13-xxx-yyy` в namespace `d8-istiod`.
  * Для каждого прикладного namespace, где включен istio:
    * Поменять label `istio-injection: enabled` на `istio.io/rev: v1x13`.
    * По очереди пересоздать Pod'ы в namespace, параллельно контролируя работоспособность приложения.
  * Поменять настройку `istio.globalVersion` на `1.13` и удалить `additionalVersions`.
  * Убедиться, что старый Pod `istiod` удалился.
  * Поменять лейблы прикладных namespace на `istio-injection: enabled`.
