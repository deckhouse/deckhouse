---
title: "ALB средствами Istio"
permalink: ru/admin/configuration/network/ingress/alb/istio.html
description: "Настройка Application Load Balancer с Istio в Deckhouse Kubernetes Platform. Настройка Istio Ingress Gateway, управление трафиком и интеграция с service mesh."
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Публикация приложений средствами Istio"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html
  - title: "Балансировка входящего трафика"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "Документация модуля istio"
    url: /modules/istio/
  - title: "Custom Resources модуля istio"
    url: /modules/istio/cr.html
---

ALB средствами Istio реализуется через [Istio Ingress Gateway](#istio-ingress-gateway) или [Ingress NGINX](#ingress-nginx). Для этого используется модуль [`istio`](/modules/istio/).

Используйте этот вариант, если требуется управление трафиком в service mesh, например, canary-маршрутизация или mTLS. Настройка и возможности описаны в [«Документация модуля istio»](/modules/istio/).

Создание IngressIstioController и подготовка инфраструктуры — задача администратора кластера. Публикация приложения ресурсами Gateway и VirtualService, в том числе [canary-развёртывание](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#canary-развёртывание-через-virtualservice), описана в разделе [«Публикация приложений средствами Istio»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway).

## Ingress для публикации приложений

### Istio Ingress Gateway {#istio-ingress-gateway}

Для публикации приложения средствами Istio Ingress Gateway выполните следующие действия:

1. Создайте ресурс IngressIstioController.

   В примере ниже создаётся контроллер с инлетом `HostPort` на frontend-узлах:

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

1. Создайте секрет с TLS-сертификатом и ключом для HTTPS. Разместите его в неймспейсе `d8-ingress-istio` и сообщите разработчикам имя секрета и значение `ingressGatewayClass`:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: app-tls-secret
     namespace: d8-ingress-istio # Неймспейс секрета — d8-ingress-istio, не неймспейс приложения.
   type: kubernetes.io/tls
   data:
     tls.crt: |
       <TLS_CRT_DATA>
     tls.key: |
       <TLS_KEY_DATA>
   ```

Поддерживаемые форматы секретов — в [документации Istio](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats).

Далее разработчики создают ресурсы Gateway и VirtualService. Примеры, в том числе [canary-развёртывание](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#canary-развёртывание-через-virtualservice), — в разделе [«Публикация приложений с использованием ресурса Istio Ingress Gateway»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway).

### Ingress NGINX {#ingress-nginx}

Для публикации через Ingress NGINX с Istio-сайдкаром включите параметр `enableIstioSidecar` в [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) модуля [`ingress-nginx`](/modules/ingress-nginx/) и сообщите разработчикам имя `ingressClass`.

Манифесты Ingress и Service с обязательными аннотациями — в разделе [«Публикация приложений с использованием Ingress NGINX»](/products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html#публикация-приложений-с-использованием-ingress-nginx).
