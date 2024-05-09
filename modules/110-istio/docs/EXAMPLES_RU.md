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
        maxConnections: 100 # Максимальное число коннектов в сторону host, суммарно для всех эндпоинтов.
      http:
        maxRequestsPerConnection: 10 # Каждые 10 запросов коннект будет пересоздаваться.
    outlierDetection:
      consecutive5xxErrors: 7 # Допустимо 7 ошибок (включая пятисотые, TCP-таймауты и HTTP-таймауты)
      interval: 5m            # в течение пяти минут,
      baseEjectionTime: 15m   # после которых эндпоинт будет исключен из балансировки на 15 минут.
```

А также для настройки HTTP-таймаутов используется ресурс [VirtualService](istio-cr.html#virtualservice). Эти таймауты также учитываются при подсчете статистики ошибок на эндпоинтах.

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

**Важно!** Чтобы балансировка gRPC-сервисов заработала автоматически, присвойте name с префиксом или значением `grpc` для порта в соответствующем Service.

## Locality Failover

> При необходимости ознакомьтесь с [основной документацией](https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/).

Istio позволяет настроить приоритетный географический фейловер между эндпоинтами. Для определения зоны Istio использует лейблы узлов с соответствующей иерархией:

* `topology.istio.io/subzone`;
* `topology.kubernetes.io/zone`;
* `topology.kubernetes.io/region`.

Это полезно для межкластерного фейловера при использовании совместно с [мультикластером](#устройство-мультикластера-из-двух-кластеров-с-помощью-ресурса-istiomulticluster).

> **Важно!** Для включения Locality Failover используется ресурс DestinationRule, в котором также необходимо настроить `outlierDetection`.

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
        enabled: true # Включили LF.
    outlierDetection: # outlierDetection включить обязательно.
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

**Важно!** Istio отвечает лишь за гибкую маршрутизацию запросов, которая опирается на спецзаголовки запросов (например, cookie) или просто на случайность. За настройку этой маршрутизации и «переключение» между канареечными версиями отвечает CI/CD-система.

Подразумевается, что в одном namespace выкачено два Deployment с разными версиями приложения. У подов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два custom resource:
* [DestinationRule](istio-cr.html#destinationrule) с описанием, как идентифицировать разные версии вашего приложения (subset'ы);
* [VirtualService](istio-cr.html#virtualservice) с описанием, как распределять трафик между разными версиями приложения.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  # subset'ы доступны только при обращении к хосту через VirtualService из пода под управлением Istio.
  # Эти subset'ы должны быть указаны в маршрутах.
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
      weight: 90 # Процент трафика, который получат поды с лейблом version: v1.
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
  # ingressGatewayClass содержит значение селектора меток, используемое при создании ресурса Gateway.
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
  namespace: d8-ingress-istio # Обратите внимание, что namespace не является app-ns.
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
    # Селектор меток для использования Istio Ingress Gateway main-hp.
    istio.deckhouse.io/ingress-gateway-class: istio-hp
  servers:
    - port:
        # Стандартный шаблон для использования протокола HTTP.
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - app.example.com
    - port:
        # Стандартный шаблон для использования протокола HTTPS.
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        # Secret с сертификатом и ключем, который должен быть создан в d8-ingress-istio namespace.
        # Поддерживаемые форматы Secret'ов можно посмотреть по ссылке https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
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

### NGINX Ingress

Для работы с NGINX Ingress требуется подготовить:
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` у custom resource [IngressNginxController](../../modules/402-ingress-nginx/cr.html#ingressnginxcontroller) модуля [ingress-nginx](../../modules/402-ingress-nginx/).
* Ingress-ресурс, который ссылается на Service. Обязательные аннотации для Ingress-ресурса:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы слать их напрямую в поды приложения. Sidecar-контейнер `istio-proxy` перехватывает трафик только в сторону диапазона Service CIDR, остальные запросы отправляются напрямую;
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — с данной аннотацией sidecar сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Просим nginx проксировать трафик на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов.
    # Чтобы не сообщать Istio о существовании внешнего домена `productpage.example.com`,
    # мы просто используем внутренний домен, о котором Istio осведомлен.
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

**Важно!** Как только для приложения создается `AuthorizationPolicy`, начинает работать следующий алгоритм принятия решения о судьбе запроса:
* Если запрос попадает под политику DENY — запретить запрос.
* Если для данного приложения нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если вы явно что-то запретили, работает только ваш запрет. Если же вы что-то явно разрешили, теперь разрешены только явно одобренные запросы (запреты никуда не исчезают и имеют приоритет).

**Важно!** Для работы политик, основанных на высокоуровневых параметрах, таких как namespace или principal, необходимо, чтобы все вовлеченные сервисы работали под управлением Istio. Также между приложениями должен быть организован Mutual TLS.

Примеры:
* Запретим POST-запросы для приложения myapp. Отныне, так как для приложения появилась политика, согласно алгоритму выше будут запрещены только POST-запросы к приложению.

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

* Здесь для приложения создана политика ALLOW. При ней будут разрешены только запросы из NS `bar`, остальные запрещены.

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
    rules:
    - from:
      - source:
          namespaces: ["bar"]
  ```

* Здесь для приложения создана политика ALLOW. При этом она не имеет ни одного правила, и поэтому ни один запрос под нее не попадет, но она таки есть. Поэтому, согласно алгоритму, раз что-то разрешено, то все остальное запрещено. В данном случае все остальное — это вообще все запросы.

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

* Здесь для приложения созданы политика ALLOW (это default) и одно пустое правило. Под это правило попадает любой запрос и автоматически получает добро.

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

### Запретить вообще все в рамках namespace foo

Два способа:

* Запретить явно. Здесь мы создаем политику DENY с единственным универсальным фильтром `{}`, под который попадают все запросы:

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

* Неявно. Здесь мы создаем политику ALLOW (по умолчанию), но не создаем ни одного фильтра, так что ни один запрос под нее не попадет и будет автоматически запрещен.

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
   - source: # Правила ниже логически перемножаются.
       namespaces: ["baz"]
       principals: ["foo.local/*", "bar.local/*"]
```

### Разрешить из любого кластера (по mTLS)

**Важно!** Если есть запрещающие правила, у них будет приоритет. Смотри [алгоритм](#алгоритм-принятия-решения).

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
       principals: ["*"] # To set mTLS mandatory.
```

### Разрешить вообще откуда угодно (в том числе без mTLS)

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

## Устройство федерации из двух кластеров с помощью custom resource IstioFederation

> Доступно только в редакции Enterprise Edition.

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

> Доступно только в редакции Enterprise Edition.

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

## Управление поведением data plane

### Предотвратить завершение работы istio-proxy до завершения соединений основного приложения

По умолчанию в процессе остановки пода все контейнеры, включая istio-proxy, получают сигнал SIGTERM одновременно. Но некоторым приложениям для правильного завершения работы необходимо время и иногда дополнительная сетевая активность. Это невозможно, если istio-proxy завершился раньше.

Решение — добавить в istio-proxy preStop-хук для оценки активности прикладных контейнеров, а единственный доступный метод — это выявление сетевых сокетов приложения, и если таковых нет, тогда можно останавливать контейнер.

Аннотация ниже добавляет описанный выше preStop-хук в контейнер istio-proxy прикладного пода:

```yaml
annotations:
  inject.istio.io/templates: "sidecar,d8-hold-istio-proxy-termination-until-application-stops"
```

## Ограничения режима перенаправления прикладного трафика для передачи под управление Istio `CNIPlugin`

В отличие от режима `InitContainer`, настройка перенаправления осуществляется в момент создании пода, а не в момент срабатывания init-контейнера `istio-init`. Это значит, что прикладные init-контейнеры не смогут взаимодействовать с остальными сервисами так как весь трафик будет перенаправлен на обработку в сайдкар `istio-proxy`, который ещё не запущен. Обходные пути:

* Запустить прикладной init-контейнер от пользователя с uid `1337`. Запросы данного пользователя не перехватываются под управление Istio.
* Исключить IP-адрес или порт сервиса из-под контроля Istio с помощью аннотаций `traffic.sidecar.istio.io/excludeOutboundIPRanges` или `traffic.sidecar.istio.io/excludeOutboundPorts`.

## Обновление Istio

## Обновление control plane Istio

* Deckhouse позволяет инсталлировать несколько версий control plane одновременно:
  * Одна глобальная, обслуживает namespace'ы или поды без явного указания версии (label у namespace `istio-injection: enabled`). Настраивается параметром [globalVersion](configuration.html#parameters-globalversion).
  * Остальные — дополнительные, обслуживают namespace'ы или поды с явным указанием версии (label у namespace или пода `istio.io/rev: v1x19`). Настраиваются параметром [additionalVersions](configuration.html#parameters-additionalversions).
* Istio заявляет обратную совместимость между data plane и control plane в диапазоне двух минорных версий:
![Istio data-plane and control-plane compatibility](https://istio.io/latest/blog/2021/extended-support/extended_support.png)
* Алгоритм обновления (для примера, на версию `1.19`):
  * Добавить желаемую версию в параметр модуля [additionalVersions](configuration.html#parameters-additionalversions) (`additionalVersions: ["1.19"]`).
  * Дождаться появления соответствующего пода `istiod-v1x19-xxx-yyy` в namespace `d8-istio`.
  * Для каждого прикладного namespace, где включен istio:
    * поменять label `istio-injection: enabled` на `istio.io/rev: v1x19`;
    * по очереди пересоздать поды в namespace, параллельно контролируя работоспособность приложения.
  * Поменять настройку `globalVersion` на `1.19` и удалить `additionalVersions`.
  * Убедиться, что старый под `istiod` удалился.
  * Поменять лейблы прикладных namespace на `istio-injection: enabled`.

Чтобы найти все поды под управлением старой ревизии Istio, выполните:

```shell
kubectl get pods -A -o json | jq --arg revision "v1x16" \
  '.items[] | select(.metadata.annotations."sidecar.istio.io/status" // "{}" | fromjson |
   .revision == $revision) | .metadata.namespace + "/" + .metadata.name'
```

### Автоматическое обновление data plane Istio

> Доступно только в редакции Enterprise Edition.

Для автоматизации обновления istio-sidecar'ов установите лейбл `istio.deckhouse.io/auto-upgrade="true"` на `Namespace` либо на отдельный ресурс — `Deployment`, `DaemonSet` или `StatefulSet`.
