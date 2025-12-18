---
title: "ALB"
permalink: ru/user/network/ingress/alb.html
lang: ru
---

Application Load Balancer (ALB) реализуется с помощью Ingress-ресурсов и Gateway.
ALB позволяет обрабатывать следующие виды трафика: HTTP, HTTPS и gRPC.
Для публикации приложений используется настроенный администратором Ingress-контроллер.
В большинстве случаев применяется модуль [ingress-nginx](/modules/ingress-nginx/), для более сложных задач может использоваться модуль [istio](/modules/istio/).

## Советы по выбору и особенности ALB средствами ingress-nginx и istio

### Ingress-nginx

ALB  [ingress-nginx](/modules/ingress-nginx/) основан на базе веб-сервера nginx.
Этот вариант подходит для:

- базовой маршрутизации трафика на основе доменов или URL;
- использования SSL/TLS для защиты трафика.

### Istio

ALB на основе [istio](/modules/istio/) позволяет получить расширенные возможности по управлению трафиком. ALB на базе istio стоит рассмотреть, если вам нужны:

- продвинутая маршрутизация, например, для реализации [canary deployment](../canary-deployment.html).
- распределение трафика между версиями приложения и микросервисами;
- mTLS для шифрования трафика между подами;
- трассировка запросов.

## Пример базового Ingress-ресурса для публикации приложения

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: lab-5-ingress
spec:
  ingressClassName: nginx
  rules:
  - host: <Указан в вашем журнале>
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: lab-5-service
            port:
              number: 80
```

## Пример ресурса Ingress NGINX

Для работы с Ingress NGINX администратор Deckhouse Kubernetes Platform должен настроить Ingress-контроллер, добавив к нему сайдкар от Istio.
Для этого установите параметр [`enableIstioSidecar`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) у кастомного ресурса IngressNginxController модуля [ingress-nginx](/modules/ingress-nginx/).

Для публикации приложения подготовьте Ingress-ресурс, который ссылается на сервис. Обязательные аннотации для Ingress-ресурса:
  
* `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы отсылать их напрямую в поды приложения. Сайдкар-контейнер `istio-proxy` перехватывает трафик только в сторону диапазона Service CIDR, остальные запросы отправляются напрямую;
* `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — с данной аннотацией сайдкар сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Включает через nginx проксирование трафика на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов.
    # Это позволяет избежать необходимости указывать Istio о существовании внешнего домена `productpage.example.com`,
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

## Пример ресурса Istio Ingress Gateway

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
        # Поддерживаемые форматы Secret можно посмотреть по ссылке https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

## Балансировка gRPC

Чтобы балансировка gRPC-сервисов заработала автоматически, присвойте имя с префиксом или значением `grpc` для порта соответствующему объекту Service.
