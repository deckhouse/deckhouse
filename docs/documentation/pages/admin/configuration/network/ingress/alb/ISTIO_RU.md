---
title: "ALB средствами Istio"
permalink: ru/admin/configuration/network/ingress/alb/istio.html
description: "Настройка Application Load Balancer с Istio в Deckhouse Kubernetes Platform. Настройка Istio Ingress Gateway, управление трафиком и интеграция с service mesh."
lang: ru
---

ALB средствами Istio реализуется через [Istio Ingress Gateway](#istio-ingress-gateway) или [Ingress NGINX](#ingress-nginx). Для этого используется модуль [`istio`](/modules/istio/).

## Ingress для публикации приложений

### Istio Ingress Gateway

Для публикации приложения средствами Istio Ingress Gateway выполните следующие действия:

1. Создайте ресурс IngressIstioController:

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

1. Создайте секрет, определяющий TLS-сертификат и ключ для защиты HTTPS-трафика:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: app-tls-secret
     namespace: d8-ingress-istio # Обратите внимание, что namespace не является app-ns.
   type: kubernetes.io/tls
   data:
     tls.crt: |
       <tls.crt data>
     tls.key: |
       <tls.key data>
   ```

1. Создайте ресурс Gateway. В нём, в поле `spec.selector` укажите лейбл, ссылающийся на ingressGatewayClass, и секрет, созданные на предыдущих шагах:

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
           credentialName: app-tls-secret
         hosts:
           - app.example.com
   ```

   Поддерживаемые форматы секретов можно посмотреть [на официальном сайте проекта Istio](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats).

1. Определите правила маршрутизации с помощью VirtualService, который связывает шлюз и обслуживаемый им сервис:

   ```yaml
   apiVersion: networking.istio.io/v1alpha3
   kind: VirtualService
   metadata:
     name: vs-app
     namespace: app-ns
   spec:
     gateways:
       - gateway-app # Имя шлюза.
     hosts:
       - app.example.com
     http:
       - route:
           - destination:
               host: app-svc # Имя сервиса.
   ```

### Ingress NGINX

Для работы с Ingress NGINX требуется подготовить:

* Ingress-контроллер, добавив к нему sidecar от Istio. Для этого установите параметр `enableIstioSidecar` кастомного ресурса [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) модуля [`ingress-nginx`](/modules/ingress-nginx/).
* Ingress-ресурс, который ссылается на сервис. Обязательные аннотации для Ingress-ресурса:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — эта аннотация указывает Ingress-контроллеру направлять запросы на ClusterIP сервиса (из диапазона Service CIDR), а не напрямую в поды приложения. Это необходимо, поскольку sidecar-контейнер `istio-proxy` перехватывает только трафик, направленный на диапазон Service CIDR. Запросы вне этого диапазона не проходят через Istio;
  * `nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc` — с данной аннотацией sidecar сможет идентифицировать прикладной сервис, для которого предназначен запрос.

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
