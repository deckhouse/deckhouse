---
title: Инлет
description: Способ поступления входящего трафика в Ingress NGINX Controller. Задаётся в поле inlet объекта [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) и принимает одно из значений LoadBalancer, LoadBalancerWithProxyProtocol, HostPort или HostWithFailover.
lang: ru
---

Инлет `LoadBalancer`, как и `LoadBalancerWithProxyProtocol`, использует балансировщик нагрузки [облачного провайдера](glossary.html#cloud-provider) или встроенный модуль [`metallb`](/modules/metallb/). Инлет `HostPort` публикует [Ingress-контроллер](glossary.html#ingress-controller) на портах узлов, например за внешним балансировщиком нагрузки. Инлет `HostWithFailover` применяется на bare metal, где контроллер работает на [frontend-узлах](glossary.html#frontend-node).

Примеры для каждого инлета приведены в разделе [«ALB средствами Ingress NGINX Controller»](admin/configuration/network/ingress/alb/nginx.html).
