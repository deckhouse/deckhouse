---
title: "The istio module: usage"
---

## Circuit Breaker

Для выявления проблемных эндпоинтов используются настройки `outlierDetection` в CR [DestinationRule](istio-cr.html#destinationrule).
Более подробно алгоритм Outlier Detection описан в [документации Envoy](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/outlier).

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

А также, для настройки HTTP-таймаутов используется ресурс [VirtualService](istio-cr.html#virtualservice). Эти таймауты также учитываются при подсчёте статистики ошибок на эндпоинтах:

```
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

Основная документация: https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/

Istio позволяет настроить приоритетный географический фейловер между эндпоинтами. Для определения зоны Istio использует лейблы нод с соответствующей иерархией:

* `topology.istio.io/subzone`
* `topology.kubernetes.io/zone`
* `topology.kubernetes.io/region`

Это полезно для межкластерного фейловера при использовании совместно с [мультикластером](readme.html#multicluster).

**Важно!** Для включения Locality Failover используется ресурс DestinationRule, в котором также необходимо настроить outlierDetection.\

```
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

**Внимание!** По умолчанию все запросы включая POST ретраятся по три раза.

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

**Важно!** Istio отвечает лишь за гибкую маршрутизацию запросов, которая опирается на спец-заголовки запросов (например, куки) или просто на случайность. За настройку этой маршрутизации и "переключение" между канареечными версиями отвечает CI/CD система.

Подразумевается, что в одном namespace выкачено два Deployment с разными версиями приложения. У Pod'ов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два custom resource:
* [DestinationRule](istio-cr.html#destinationrule) с описанием, как идентифицировать разные версии вашего приложения (subset-ы).
* [VirtualService](istio-cr.html#virtualservice) с описанием, как распределять трафик между разными версиями приложения.

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  subsets: # subset-ы доступны только при обращении к хосту через через VirtualService из пода под управлением Istio. Эти subset-ы должны быть указаны в маршрутах.
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

### Распределение по наличию cookie

```
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

## Ingress

To use Ingress, you need to:
* Configure the Ingress controller by adding Istio sidecar to it. In our case, you need to enable the `enableIstioSidecar` parameter in the [ingress-nginx](../../modules/402-ingress-nginx/) module's [IngressNginxController](../../modules/402-ingress-nginx/cr.html#ingressnginxcontroller) custom resource.
* Set up an Ingress that refers to the Service. The following annotations are mandatory for Ingress:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — using this annotation, the Ingress controller sends requests to a single ClusterIP (from Service CIDR) while envoy load balances them. Ingress controller's sidecar is only catching traffic directed to Service CIDR.
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — using this annotation, the sidecar can identify the application service that serves requests.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    nginx.ingress.kubernetes.io/service-upstream: "true" # Nginx proxies traffic to the ClusterIP instead of pods' own IPs.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc # In Istio, all routing is carried out based on the `Host:` headers. Instead of letting Istio know about the `productpage.example.com` external domain, we use the internal domain of which Istio is aware.
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

## Authorization configuration examples

### Decision-making algorithm

**Caution!** The following algorithm for deciding the fate of a request becomes active after AuthorizationPolicy is created for the application:
* The request is denied if it falls under the DENY policy;
* The request is allowed if there are no ALLOW policies for the application;
* The request is allowed if it falls under the ALLOW policy.
* All other requests are denied.

In other words, if you explicitly deny something, then only this restrictive rule will work. If you explicitly allow something, only explicitly authorized requests will be allowed (however, restrictions will stay in force and have precedence).

**Caution!** The policies based on high-level parameters like namespace or principal require enabling Istio for all involved applications. Also, there must be organized Mutual TLS between applications, by default it is, due module configuration parameter `tlsMode: MutualPermissive`.

Examples:
* Let's deny POST requests for the myapp application. Since a policy is defined, only POST requests to the application are denied (as per the algorithm above).

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

* Below, the ALLOW policy is defined for the application. It only allows requests from the `bar` namespace (other requests are denied).

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
    action: ALLOW # The default value, can be skipped.
    rules:
    - from:
      - source:
          namespaces: ["bar"]
  ```

* Below, the ALLOW policy is defined for the application. Note that it does not have any rules, so not a single request matches it (still, the policy exists). Thus, our decision-making algorithm suggests that if something is allowed, then everything else is denied. In this case, "everything else" includes all the requests.

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
    action: ALLOW # The default value, can be skipped.
    rules: []
  ```

* Below, the (default) ALLOW policy is defined for the application. Note that it has an empty rule. Any request matches this rule, so it is naturally approved.

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

### Deny all for the foo namespace

There are two ways you can do that:

* Explicitly. Here, the DENY policy is created. It has a single `{}` rule that covers all the requests:

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

* Implicitly. Here, the (default) ALLOW policy is created that does not have any rules. Thus, no requests will match it, and the policy will deny all of them.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec: {}
  ```

### Deny requests from the foo NS only

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

### Allow requests for the foo NS only

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

### Allow requests from anywhere in the cluster

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

### Allow any requests for foo or bar clusters

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

### Allow any requests from foo or bar clusters where the namespace is baz

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
   - source: # Logical conjunction is used for the rules below.
       namespaces: ["baz"]
       principals: ["foo.local/*", "bar.local/*"]
```

### Allow from any cluster (via mtls)

**Caution!** The denying rules (if they exist) have priority over any other rules. See the [algorithm](#decision-making-algorithm).

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
       principals: ["*"] # To force the MTLS usage.
```

### Allow all requests from anywhere (including no mTLS - plain text traffic)

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

сluster A:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```

сluster B:
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

cluster A:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

cluster B:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

## Upgrading Istio control-plane

* Deckhouse allows you to install different control-plane versions simultaneously:
  * A single global version to handle namespaces or Pods with indifferent version (namespace label `istio-injection: enabled`). It is configured by `istio.globalVersion` mandatory argument in the `deckhouse` ConfigMap.
  * The other ones are additional, they handle namespaces or Pods with explicitly configured versions (`istio.io/rev: v1x13` label for namespace or Pod). They are configured by `istio.additionalVersions` argument in the `deckhouse` ConfigMap.
* Istio declares backward compatibility between data-plane and control-plane in the range of two minor versions:
![Istio data-plane and control-plane compatibility](https://istio.io/latest/blog/2021/extended-support/extended_support.png)
* Upgrade algorithm (i.e. to `1.13`):
  * Confugure additional version in CM deckhouse (`additionalVersions: ["1.13"]`).
  * Wait for the corresponding pod `istiod-v1x13-xxx-yyy` to appear in `d8-istiod` namespace.
  * For every application Namespase with istio enabled:
    * Change `istio-injection: enabled` lable to `istio.io/rev: v1x13`.
    * Recreate the Pods in namespace (one at a time), simultaneously monitoring the application workability.
  * Reconfigure `istio.globalVersion` to `1.13` and remove the `additionalVersions` configuration.
  * Make sure, the old `istiod` Pod has gone.
  * Change application namespace labels to `istio-injection: enabled`.

Find all pods with old Istio revision:
```
kubectl get pods -A -o json | jq --arg revision "v1x12" '.items[] | select(.metadata.annotations."sidecar.istio.io/status" // "{}" | fromjson | .revision == $revision) | .metadata.namespace + "/" + .metadata.name'
```
