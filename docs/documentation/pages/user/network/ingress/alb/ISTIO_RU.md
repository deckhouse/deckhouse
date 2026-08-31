---
title: "Публикация приложений средствами Istio"
description: "Публикация приложений с Istio в Deckhouse Kubernetes Platform. Ingress NGINX с Istio-сайдкаром и Istio Ingress Gateway с ресурсами Gateway и VirtualService."
permalink: ru/user/network/ingress/alb/istio.html
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Использование Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB средствами Istio"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html
  - title: "Документация модуля istio"
    url: /modules/istio/
---

## Публикация приложений средствами Istio

Публикация средствами Istio настраивается в два слоя. Администратор кластера разворачивает IngressIstioController (и связанную инфраструктуру) в разделе [«Istio Ingress Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html#istio-ingress-gateway). Разработчики приложения создают ресурсы Gateway и VirtualService, как показано ниже.

При публикации приложения средствами Istio можно выбрать один из вариантов:

- [«Использование Ingress NGINX»](#публикация-приложений-с-использованием-ingress-nginx).
- [«Использование Istio Ingress Gateway»](#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway).

### Публикация приложений с использованием Ingress NGINX {#публикация-приложений-с-использованием-ingress-nginx}

Для публикации приложения средствами Ingress NGINX администратор DKP должен настроить Ingress-контроллер, добавив к нему сайдкар от Istio.

Для публикации приложения подготовьте Ingress-ресурс, который ссылается на сервис. Обязательные аннотации для Ingress-ресурса:
  
- `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы отправлять их напрямую в поды приложения. Сайдкар-контейнер `istio-proxy` перехватывает трафик только в сторону диапазона Service CIDR, остальные запросы отправляются напрямую.
- `nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc` — с этой аннотацией сайдкар сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Включает проксирование трафика через nginx на ClusterIP вместо собственных IP подов.
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

### Публикация приложений с использованием ресурса Istio Ingress Gateway {#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway}

Для публикации приложения средствами Istio Ingress Gateway администратор DKP должен создать ресурс IngressIstioController.

Для публикации приложения с использованием ресурса Istio Ingress Gateway:

1. Создайте ресурс Gateway (шлюз). В нём, в поле `spec.selector` укажите лейбл, ссылающийся на ingressGatewayClass, и имя секрета, полученные от администратора кластера:

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
           # Ресурс Secret с сертификатом и ключом, который должен быть создан администратором в неймспейсе d8-ingress-istio.
           # Поддерживаемые форматы Secret можно посмотреть по ссылке https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
           credentialName: app-tls-secret
         hosts:
           - app.example.com
   ```

1. Определите правила маршрутизации с помощью VirtualService, который связывает шлюз и обслуживаемый им сервис:

   ```yaml
   apiVersion: networking.istio.io/v1beta1
   kind: VirtualService
   metadata:
     name: vs-app
     namespace: app-ns
   spec:
     gateways:
       - gateway-app # Имя ресурса Gateway, созданного на предыдущем шаге.
     hosts:
       - app.example.com
     http:
       - route:
           - destination:
               host: app-svc # Имя сервиса, на который нужно направить трафик.
   ```
