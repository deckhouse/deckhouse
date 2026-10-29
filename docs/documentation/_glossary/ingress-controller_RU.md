---
title: Ingress-контроллер
description: Контроллер, который маршрутизирует запросы пользователей к приложениям и сервисам по правилам, описанным в объектах Ingress.
lang: ru
---

В Deckhouse Platform Ingress-контроллеры управляются модулями и кастомными ресурсами. Доступные реализации: Ingress NGINX Controller (модуль [`ingress-nginx`](/modules/ingress-nginx/)), Kubernetes Gateway API (модуль [`alb`](/modules/alb/)) и Istio (модуль [`istio`](/modules/istio/)). В одном кластере могут одновременно работать несколько Ingress NGINX Controller, один из них основной.

Ingress-контроллеры обычно размещают на [frontend-узлах](glossary.html#frontend-node). Способ поступления трафика в контроллер определяет его [инлет](glossary.html#inlet). Подробнее в разделе [«Балансировка входящего трафика»](admin/configuration/network/ingress/).
