---
title: Инлет
description: Способ поступления входящего трафика в балансировщик уровня приложений — Ingress NGINX Controller или шлюз модуля [`alb`](/modules/alb/). Задаётся в поле inlet объектов [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller), [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) и [ALBInstance](/modules/alb/cr.html#albinstance).
lang: ru
---

Для [Ingress-контроллера](glossary.html#ingress-controller) инлет принимает одно из значений `LoadBalancer`, `LoadBalancerWithProxyProtocol`, `HostPort` или `HostWithFailover`. Инлеты `LoadBalancer` и `LoadBalancerWithProxyProtocol` используют балансировщик нагрузки [облачного провайдера](glossary.html#cloud-provider) или встроенный модуль [`metallb`](/modules/metallb/). Инлет `HostPort` публикует контроллер на портах узлов, например за внешним балансировщиком нагрузки. Инлет `HostWithFailover` применяется на bare metal, где контроллер работает на [frontend-узлах](glossary.html#frontend-node).

Для шлюзов модуля `alb` инлет принимает одно из значений `LoadBalancer`, `HostPort` или `ClusterIP`. Для ClusterALBInstance доступны `LoadBalancer` и `HostPort`, для ALBInstance — `LoadBalancer` и `ClusterIP`. Инлет `ClusterIP` принимает трафик через сервис с типом `ClusterIP`, без внешней точки входа.

Примеры для каждого инлета приведены в разделах [«ALB средствами Ingress NGINX Controller»](admin/configuration/network/ingress/alb/nginx.html) и [«Инлеты»](admin/configuration/network/ingress/alb/alb-gateway-api.html#инлеты) страницы «ALB средствами Kubernetes Gateway API».
