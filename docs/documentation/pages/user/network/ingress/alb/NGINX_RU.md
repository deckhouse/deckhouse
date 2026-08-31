---
title: "Публикация приложений средствами Ingress NGINX Controller"
description: "Публикация приложений с Ingress NGINX Controller в Deckhouse Kubernetes Platform. Примеры ресурса Ingress и балансировка gRPC."
permalink: ru/user/network/ingress/alb/nginx.html
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Использование Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB средствами Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "Документация модуля ingress-nginx"
    url: /modules/ingress-nginx/
  - title: "Custom Resources модуля ingress-nginx"
    url: /modules/ingress-nginx/cr.html
  - title: "Примеры модуля ingress-nginx"
    url: /modules/ingress-nginx/examples.html
---

## Публикация приложений средствами Ingress NGINX Controller

Для публикации приложений администратор кластера должен создать Ingress-контроллер. Порядок настройки описан в разделе [«Примеры настройки балансировки»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html#load-balancing-configuration-examples). Запросите у администратора имя `ingressClass` контроллера и укажите его в манифесте ресурса Ingress, который маршрутизирует входящий трафик к приложению.

Пример базового Ingress-ресурса для публикации приложения.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
spec:
  ingressClassName: nginx # Имя Ingress-контроллера, предоставленного администратором кластера.
  rules:
  - host: application.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: productpage
            port:
              number: 80
```

## Балансировка gRPC

Раздел относится к публикации gRPC через Ingress NGINX Controller (`ingress-nginx`). Для Gateway API используйте [GRPCRoute](gateway-api.html#grpcroute-tlsroute-tcproute-and-udproute-objects).

Чтобы балансировка gRPC-сервисов за Ingress NGINX заработала автоматически, присвойте имя с префиксом или значением `grpc` порту соответствующего объекта Service.
