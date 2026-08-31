---
title: "Использование Application Load Balancer (ALB)"
description: "Настройка Application Load Balancer для HTTP/HTTPS/gRPC трафика в Deckhouse Kubernetes Platform. Использование ingress-nginx, alb (Gateway API) и istio для маршрутизации запросов, терминации SSL/TLS и публикации приложений."
permalink: ru/user/network/ingress/alb.html
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Публикация приложений средствами Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/gateway-api.html
  - title: "Публикация приложений средствами Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/nginx.html
  - title: "Публикация приложений средствами Istio"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb/istio.html
  - title: "ALB средствами Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "ALB средствами Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "Миграция с ingress-nginx на alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "ALB средствами Istio"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html
  - title: "Балансировка входящего трафика"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/
  - title: "Документация модуля alb"
    url: /modules/alb/
  - title: "Документация модуля ingress-nginx"
    url: /modules/ingress-nginx/
  - title: "Документация модуля istio"
    url: /modules/istio/
---

Публикация приложений и балансировка трафика на прикладном уровне в Deckhouse Kubernetes Platform (DKP) может выполняться средствами:

- [Ingress NGINX Controller](alb/nginx.html) (модуль `ingress-nginx`).
- [Kubernetes Gateway API](alb/gateway-api.html) (модуль `alb`).
- [Istio](alb/istio.html) (модуль `istio`).

## Сравнение вариантов ALB

### Ingress NGINX

ALB средствами Ingress NGINX Controller построена на базе веб-сервера nginx и реализуется модулем [`ingress-nginx`](/modules/ingress-nginx/).
Этот вариант подходит для:

- базовой маршрутизации трафика на основе доменов или URL;
- использования SSL/TLS для защиты трафика.

### Kubernetes Gateway API

ALB средствами [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) реализуется [модулем `alb`](/modules/alb/). Шлюзы работают на Envoy Proxy. Приём и маршрутизация описываются стандартными объектами API (Gateway, ListenerSet, HTTPRoute и при необходимости GRPCRoute, TLSRoute, TCPRoute, UDPRoute, BackendTLSPolicy). Контроллер разворачивает инфраструктуру входа и проверяет конфигурацию, чтобы не допускать конфликтующих обработчиков.

Модель Gateway API разделяет ответственность между администратором кластера (ClusterALBInstance), администратором неймспейса (ALBInstance и ListenerSet — hostname, TLS, порты) и разработчиками приложения (HTTPRoute и другие ресурсы маршрутизации).

Используйте этот вариант для:

- публикации приложений в модели Gateway API вместо классического Ingress;
- общекластерной точки входа или отдельного шлюза для приложения или команды в своём неймспейсе;
- маршрутизации HTTP/HTTPS, gRPC, TCP, UDP, а также терминации или сквозной передачи TLS;
- WAF на уровне маршрута или Istio-сайдкара на прокси шлюза;
- GeoIP и OpenTelemetry на шлюзе (настраивает администратор — [«Использование GeoIP и GeoLite2»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#geoip) и [«Настройка трассировки OpenTelemetry»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#tracing));
- параметров маршрута, которых нет в спецификации, через [аннотации HTTPRoute](alb/gateway-api.html#поддерживаемые-аннотации-httproute).

Сравнение с `ingress-nginx` и пояснения по терминологии — в разделе [«Сравнение возможностей модулей ingress-nginx и alb»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/#сравнение-возможностей-модулей-ingress-nginx-и-alb).

### Istio

ALB на основе модуля [`istio`](/modules/istio/) поддерживает управление трафиком в service mesh. Используйте этот вариант для:

- маршрутизации для [canary deployment](../canary-deployment.html) и аналогичных сценариев;
- распределения трафика между версиями приложения и микросервисами;
- mTLS для шифрования трафика между подами;
- трассировки запросов.

## Как понять, что доступно в кластере

Перед публикацией приложения проверьте, какие механизмы ALB включены и настроены:

1. Убедитесь, что нужный модуль включён:

   ```shell
   d8 k get moduleconfig ingress-nginx alb istio
   ```

1. Для Ingress NGINX — посмотрите ресурсы IngressNginxController и имя IngressClass:

   ```shell
   d8 k get ingressnginxcontrollers
   d8 k get ingressclass
   ```

1. Для Gateway API — убедитесь, что есть готовый ClusterALBInstance или ALBInstance, затем найдите управляемый Gateway и ListenerSet:

   ```shell
   d8 k get clusteralbinstances,albinstances --all-namespaces
   d8 k get gateway,listenerset --all-namespaces
   ```

1. Для Istio — проверьте IngressIstioController и класс ingress gateway, который сообщит администратор кластера:

   ```shell
   d8 k get ingressistiocontrollers
   ```

Запросите у администратора кластера `ingressClass`, имя и неймспейс Gateway или класс Istio ingress для манифестов приложения.

## Следующие шаги

- Публикация приложения:
  - средствами [Kubernetes Gateway API](alb/gateway-api.html#publishing-with-listenerset-and-httproute) (модуль `alb`);
  - средствами [Ingress NGINX Controller](alb/nginx.html) (модуль `ingress-nginx`);
  - средствами [Istio](alb/istio.html) (модуль `istio`).
- Настройка инфраструктуры — руководства администратора:
  - [ALB средствами Kubernetes Gateway API](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html#создание-управляемого-объекта-gateway);
  - [ALB средствами Ingress NGINX Controller](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html#load-balancing-configuration-examples);
  - [ALB средствами Istio](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html#istio-ingress-gateway).
- Миграция с `ingress-nginx` на `alb` — [«Миграция с ingress-nginx на alb»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html).
