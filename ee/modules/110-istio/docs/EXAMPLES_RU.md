---
title: "Модуль istio: примеры"
---

## Circuit Breaker

Для выявления проблемных эндпоинтов используются настройки `outlierDetection` в custom resource [DestinationRule](istio-cr.html#destinationrule).
Более подробно алгоритм Outlier Detection описан в [документации Envoy](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier).

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: reviews-cb-policy
spec:
  host: reviews.prod.svc.cluster.local
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 100 # Максимальное число коннектов в сторону host, суммарно для всех эндпоинтов
      http:
        maxRequestsPerConnection: 10 # Каждые 10 запросов коннект будет пересоздаваться
    outlierDetection:
      consecutive5xxErrors: 7 # Допустимо 7 ошибок (включая пятисотые, TCP-таймауты и HTTP-таймауты)
      interval: 5m            # В течение пяти минут
      baseEjectionTime: 15m   # После которых эндпоинт будет исключён из балансировки на 15 минут.
```

А также, для настройки HTTP-таймаутов используется ресурс [VirtualService](istio-cr.html#virtualservice). Эти таймауты также учитываются при подсчёте статистики ошибок на эндпоинтах.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: my-productpage-rule
  namespace: myns
spec:
  hosts:
  - productpage
  http:
  - timeout: 5s
    route:
    - destination:
        host: productpage
```

## Балансировка gRPC

**Важно!** Для того, чтобы балансировка gRPC-сервисов заработала автоматически, присвойте name с префиксом или значением `grpc` для порта в соответствующем Service.

## Locality Failover

> При необходимости ознакомьтесь с [основной документацией](https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/).

Istio позволяет настроить приоритетный географический фейловер между эндпоинтами. Для определения зоны Istio использует лейблы узлов с соответствующей иерархией:

* `topology.istio.io/subzone`
* `topology.kubernetes.io/zone`
* `topology.kubernetes.io/region`

Это полезно для межкластерного фейловера при использовании совместно с [мультикластером](#устройство-мультикластера-из-двух-кластеров-с-помощью-ресурса-istiomulticluster).

> **Важно!** Для включения Locality Failover используется ресурс DestinationRule, в котором также необходимо настроить outlierDetection.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: helloworld
spec:
  host: helloworld
  trafficPolicy:
    loadBalancer:
      localityLbSetting:
        enabled: true # включили LF
    outlierDetection: # outlierDetection включить обязательно
      consecutive5xxErrors: 1
      interval: 1s
      baseEjectionTime: 1m
```

## Retry

С помощью ресурса [VirtualService](istio-cr.html#virtualservice) можно настроить Retry для запросов.

**Внимание!** По умолчанию при возникновении ошибок все запросы (включая POST-запросы) выполняются повторно до трех раз.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: ratings-route
spec:
  hosts:
  - ratings.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: ratings.prod.svc.cluster.local
    retries:
      attempts: 3
      perTryTimeout: 2s
      retryOn: gateway-error,connect-failure,refused-stream
```

## Canary

**Важно!** Istio отвечает лишь за гибкую маршрутизацию запросов, которая опирается на спец-заголовки запросов (например, cookie) или просто на случайность. За настройку этой маршрутизации и "переключение" между канареечными версиями отвечает CI/CD система.

Подразумевается, что в одном namespace выкачено два Deployment с разными версиями приложения. У Pod'ов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два custom resource:
* [DestinationRule](istio-cr.html#destinationrule) с описанием, как идентифицировать разные версии вашего приложения (subset-ы).
* [VirtualService](istio-cr.html#virtualservice) с описанием, как распределять трафик между разными версиями приложения.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  # subset-ы доступны только при обращении к хосту через через VirtualService из Pod'а под управлением Istio.
  # Эти subset-ы должны быть указаны в маршрутах.
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

### Распределение по наличию cookie

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - match:
    - headers:
       cookie:
         regex: "^(.*;?)?(canary=yes)(;.*)?"
    route:
    - destination:
        host: productpage
        subset: v2 # Ссылка на subset из DestinationRule.
  - route:
    - destination:
        host: productpage
        subset: v1
```

### Распределение по вероятности

```yaml
apiVersion: networking.istio.io/v1beta1
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

## Ingress для публикации приложений

### Istio Ingress Gateway

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
 name: main
spec:
  # ingressGatewayClass содержит значение селектора меток, используемое при создании ресурса Gateway
  ingressGatewayClass: istio-hp
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  nodeSelector:
    node-role/frontend: ''
  tolerations:
    - effect: NoExecute
      key: dedicated
      operator: Equal
      value: frontend
  resourcesRequests:
    mode: VPA
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-tls-secert
  namespace: d8-ingress-istio # обратите внимание, что namespace не является app-ns
type: kubernetes.io/tls
data:
  tls.crt: |
    <tls.crt data>
  tls.key: |
    <tls.key data>
```

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: gateway-app
  namespace: app-ns
spec:
  selector:
    # селектор меток для использования Istio Ingress Gateway main-hp
    istio.deckhouse.io/ingress-gateway-class: istio-hp
  servers:
    - port:
        # стандартный шаблон для использования протокола HTTP
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - app.example.com
    - port:
        # стандартный шаблон для использования протокола HTTPS
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        # секрет с сертификатом и ключем, который должен быть создан в d8-ingress-istio namespace
        # поддерживаемые форматы секретов можно посмотреть по ссылке https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: vs-app
  namespace: app-ns
spec:
  gateways:
    - gateway-app
  hosts:
    - app.example.com
  http:
    - route:
        - destination:
            host: app-svc
```

### Nginx Ingress

Для работы с Nginx Ingress требуется подготовить:
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` у custom resource [IngressNginxController](../../modules/402-ingress-nginx/cr.html#ingressnginxcontroller) модуля [ingress-nginx](../../modules/402-ingress-nginx/).
* Ingress-ресурс, который ссылается на Service. Обязательные аннотации для Ingress-ресурса:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы слать их напрямую в Pod'ы приложения. Sidecar-контейнер `istio-proxy` перехватывает трафик только в сторону диапазона ServiceCIDR, остальные запросы отправляются напрямую.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — с данной аннотацией sidecar сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Просим nginx проксировать трафик на ClusterIP вместо собственных IP Pod'ов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов. 
    # Чтобы не сообщать Istio о существовании внешнего домена `productpage.example.com`, 
    # мы просто используем внутренний домен, о котором Istio осведомлён.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc
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

Иными словами, если вы явно что-то запретили, то работает только ваш запрет. Если же вы что-то явно разрешили, то теперь разрешены только явно одобренные запросы (запреты никуда не исчезают и имеют приоритет).

**Важно!** Для работы политик, основанных на высокоуровневых параметрах, таких как namespace или principal, необходимо, чтобы все вовлечённые сервисы работали под управлением Istio. Также, между приложениями должен быть организован Mutual TLS.

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

**Важно!** Если есть запрещающие правила, то у них будет приоритет. Смотри [алгоритм](#алгоритм-принятия-решения).

Пример:

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

## Устройство федерации из двух кластеров с помощью CR IstioFederation

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```

## Устройство мультикластера из двух кластеров с помощью ресурса IstioMulticluster

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

## Управление поведением data-plane

### [экспериментальная функция] Предотвратить завершение работы istio-proxy до завершения соединений основного приложения

По умолчанию, в процессе остановки пода, все контейнеры, включая istio-proxy, получают сигнал SIGTERM одновременно. Но некоторым приложениям для правильного завершения работы необходимо время и иногда дополнительная сетевая активность. Это невозможно если istio-proxy завершился раньше.
Решение — добавить в istio-proxy preStop-хук для оценки активности прикладных контейнеров, а единственный доступный метод — это выявление сетевых сокетов приложения, и если таковых нет, то можно останавливать контейнер.
Аннотация ниже добавляет описанный выше preStop-хук в контейнер istio-proxy прикладного пода:
`inject.istio.io/templates: sidecar,d8-hold-istio-proxy-termination-until-application-stops`.

## Обновление Istio

## Обновление control-plane Istio

* Deckhouse позволяет инсталлировать несколько версий control-plane одновременно:
  * Одна глобальная, обслуживает namespace'ы или Pod'ы без явного указания версии (label у namespace `istio-injection: enabled`). Настраивается параметром [globalVersion](configuration.html#parameters-globalversion).
  * Остальные — дополнительные, обслуживают namespace'ы или Pod'ы с явным указанием версии (label у namespace или у Pod `istio.io/rev: v1x16`). Настраиваются параметром [additionalVersions](configuration.html#parameters-additionalversions).
* Istio заявляет обратную совместимость между data-plane и control-plane в диапазоне двух минорных версий:
![Istio data-plane and control-plane compatibility](https://istio.io/latest/blog/2021/extended-support/extended_support.png)
* Алгоритм обновления (для примера, на версию `1.16`):
  * Добавить желаемую версию в параметр модуля [additionalVersions](configuration.html#parameters-additionalversions) (`additionalVersions: ["1.16"]`).
  * Дождаться появления соответствующего Pod'а `istiod-v1x16-xxx-yyy` в namespace `d8-istiod`.
  * Для каждого прикладного namespace, где включен istio:
    * Поменять label `istio-injection: enabled` на `istio.io/rev: v1x16`.
    * По очереди пересоздать Pod'ы в namespace, параллельно контролируя работоспособность приложения.
  * Поменять настройку `globalVersion` на `1.16` и удалить `additionalVersions`.
  * Убедиться, что старый Pod `istiod` удалился.
  * Поменять лейблы прикладных namespace на `istio-injection: enabled`.

Чтобы найти все Pod'ы под управлением старой ревизии Istio, выполните:

```shell
kubectl get pods -A -o json | jq --arg revision "v1x13" \
  '.items[] | select(.metadata.annotations."sidecar.istio.io/status" // "{}" | fromjson | 
   .revision == $revision) | .metadata.namespace + "/" + .metadata.name'
```

### Автоматическое обновление data-plane Istio

Для автоматизации обновления istio-sidecar'ов установите лейбл `istio.deckhouse.io/auto-upgrade="true"` на `Namespace` либо на отдельный ресурс, `Deployment`, `DaemonSet` или `StatefulSet`.
