---
title: "ALB средствами Istio"
permalink: ru/admin/configuration/network/ingress/alb/istio.html
description: "Настройка Application Load Balancer с Istio в Deckhouse Kubernetes Platform. Настройка Istio Ingress Gateway, управление трафиком и интеграция с service mesh."
lang: ru
---

ALB средствами Istio реализуется через Istio Ingress Gateway или Ingress NGINX. Для этого используется модуль [`istio`](/modules/istio/).

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
  # ingressGatewayClass содержит значение селектора лейблов, используемое при создании ресурса Gateway.
  ingressGatewayClass: istio-hp
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
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
    # Селектор лейблов для использования Istio Ingress Gateway main-hp.
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
        # Ресурс Secret с сертификатом и ключом, который должен быть создан в пространстве имен d8-ingress-istio.
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

Поддерживаемые форматы Secret'ов можно посмотреть [на официальном сайте проекта Istio](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats).

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

### Ingress NGINX

Для работы с Ingress NGINX требуется подготовить:

* Ingress-контроллер, добавив к нему sidecar от Istio. Для этого установите параметр `enableIstioSidecar` кастомного ресурса [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) модуля [`ingress-nginx`](/modules/ingress-nginx/).
* Ingress-ресурс, который ссылается на Service. Обязательные аннотации для Ingress-ресурса:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — эта аннотация указывает Ingress-контроллеру направлять запросы на ClusterIP сервиса (из диапазона Service CIDR), а не напрямую в поды приложения. Это необходимо, поскольку sidecar-контейнер `istio-proxy` перехватывает только трафик, направленный на диапазон Service CIDR. Запросы вне этого диапазона не проходят через Istio;
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — с данной аннотацией sidecar сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Nginx будет проксировать трафик на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio маршрутизация основана на заголовке запросов `Host:`.
    # Чтобы не указывать наличие внешнего домена `productpage.example.com`,
    # используется внутренний домен, известный Istio.
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
