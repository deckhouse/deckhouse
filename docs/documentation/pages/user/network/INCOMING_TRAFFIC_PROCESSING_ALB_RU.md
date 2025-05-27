---
title: "ALB"
permalink: ru/user/network/incoming-traffic-processing-alb.html
lang: ru
---

ALB (Application Load Balancer) реализуется посредством Ingress-ресурсов и Gateway.
ALB позволяет обрабатывать трафик HTTP, HTTPs и gRPC.
Для публикации приложений используется настроенный администратором Ingress-контроллер.
Для большинства случаев подходит ingress-nginx, для специфических задач можете использовать istio.

## Советы по выбору и особенности ALB средствами ingress-nginx и istio

ALB [ingress-nginx](../reference/mc/ingress-nginx/) основан на веб-сервере NGINX. Это — простой и эффективный способ управления трафиком.
Этот вариант подходит для большинства случаев, в частности, когда нужна базовая маршрутизация трафика на основе доменов или URL.
Также ALB средствами ingress-nginx подходит для случаев, когда нужно использовать SSL/TLS для защиты трафика.

ALB на основе [istio](../reference/mc/istio/) позволяет получить расширенные возможности по управлению трафиком. Его использование стоит рассмотреть, если вам нужна продвинутая маршрутизация, например, для реализации [Canary deployment](../user/canary-deployment.html), разделение трафика между несколькими версиями приложения, mTLS для шифрования трафика между подами, распределение трафика между микросервисами, трассировка запросов.

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

## Пример ресурса NGINX Ingress

Для работы с NGINX Ingress администратор Deckhouse Kubernetes Platform должен подготовить Ingress-контроллер, добавив к нему sidecar от Istio. Для этого он включает параметр `enableIstioSidecar` у кастомного ресурса [IngressNginxController](../reference/cr/ingressnginxcontroller/) модуля [ingress-nginx](../reference/mc/ingress-nginx/).

Для публикации приложения нужно подготовить Ingress-ресурс, который ссылается на Service. Обязательные аннотации для Ingress-ресурса:
  
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

## Пример ресурса Istio Ingress Gateway

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

## Балансировка gRPC

Важно! Чтобы балансировка gRPC-сервисов заработала автоматически, присвойте name с префиксом или значением `grpc` для порта в соответствующем Service.
