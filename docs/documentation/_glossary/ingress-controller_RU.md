---
title: Ingress-контроллер
description: Контроллер, который маршрутизирует запросы пользователей к приложениям и сервисам по правилам, описанным в объектах Ingress.
lang: ru
---

В Deckhouse Platform Ingress-контроллер — это Ingress NGINX Controller, которым управляет модуль [`ingress-nginx`](/modules/ingress-nginx/) через объекты [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller). В одном кластере могут одновременно работать несколько Ingress NGINX Controller, один из них основной. Kubernetes Gateway API (модуль [`alb`](/modules/alb/)) и [Istio Ingress Gateway](glossary.html#istio-ingress-gateway) тоже балансируют трафик на уровне приложений, но работают не с объектами Ingress, а с маршрутами Gateway API и объектами Gateway и VirtualService. Все три решения — реализации [ALB](glossary.html#alb).

Ingress-контроллеры обычно размещают на [frontend-узлах](glossary.html#frontend-node). Способ поступления трафика в контроллер определяет его [инлет](glossary.html#inlet). Подробнее в разделе [«Балансировка входящего трафика»](admin/configuration/network/ingress/).
