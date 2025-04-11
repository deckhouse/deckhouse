---
title: "ALB средствами Istio"
permalink: ru/admin/network/alb-istio.html
lang: ru
---

ALB средствами Istio реализуется через Istio Ingress Gateway или NGINX Ingress. Для этого используется модуль [istio](../../reference/mc/istio/).

<!-- Перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#ingress-%D0%B4%D0%BB%D1%8F-%D0%BF%D1%83%D0%B1%D0%BB%D0%B8%D0%BA%D0%B0%D1%86%D0%B8%D0%B8-%D0%BF%D1%80%D0%B8%D0%BB%D0%BE%D0%B6%D0%B5%D0%BD%D0%B8%D0%B9 -->

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
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` у custom resource [IngressNginxController](../../reference/cr/ingressnginxcontroller/) модуля [ingress-nginx](../../reference/mc/ingress-nginx/).
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
