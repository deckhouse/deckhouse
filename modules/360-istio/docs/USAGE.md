---
title: "Модуль istio: примеры использования"
---

## Circuit Breaker

Для одного сервиса потребуется единственный CR [DestinationRule](cr.html#destinationrule).

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: reviews-circuit-breaker
spec:
  host: reviews.prod.svc.cluster.local # либо полный fqdn, либо локальный для namespace домен.
  trafficPolicy:
    outlierDetection:
      consecutiveErrors: 7 # можно допустить не более семи ошибок
      interval: 5m # в течение пяти минут,
      baseEjectionTime: 15m # при этом проблемный эндпоинт будет исключён из работы на 15 минут.
```

## Retry

Для одного сервиса потребуется единственный CR [VirtualService](cr.html#virtualservice).

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: productpage-retry
spec:
  hosts:
    - productpage # либо полный fqdn, либо локальный для namespace домен.
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

Требуется настроить два custom resource'а:
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

## Ingress

Для работы с Ingress требуется подготовить:
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` модуля [nginx-ingress](/modules/400-nginx-ingress).
* Service, на который будет ссылаться Ingress. Обязательно с `ClusterIP`.
* Ingress, который ссылается на Service. У Ingress должна быть аннотация `nginx.ingress.kubernetes.io/service-upstream: "true"`. Sidecar-ы от Istio, которые прикреплены к ingress, перехватывают только трафик, адресованный на диапазон Service CIDR, соответственно, мы получаем возможность разделить два мира. В классическом мире ingress обращается напрямую к подам на диапазон Pod CIDR и всё работает как прежде. В мире же Istio, ingress обращается на ClusterIP и тем самым трафик перехватывается sidecar-ом.

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    nginx.ingress.kubernetes.io/service-upstream: "true" # Просим nginx проксировать трафик на ClusterIP вместо собственных IP подов. Трафик на диапазон ClusterIP перехватывает Istio, а трафик на CIDR подов работает по-старому.
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
  type: ClusterIP # Обязательно!
```

