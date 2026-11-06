---
title: ALB (балансировка на уровне приложений)
description: Балансировка нагрузки на уровне приложений. Анализируются заголовки HTTP(S), пути и домены, поддерживаются SSL-терминация и маршрутизация по содержимому.
lang: ru
---

Deckhouse Platform предоставляет три реализации ALB: Ingress NGINX Controller (модуль [`ingress-nginx`](/modules/ingress-nginx/)), Kubernetes Gateway API (модуль [`alb`](/modules/alb/)) и Istio (модуль [`istio`](/modules/istio/)). Балансировка на сетевом уровне — это [NLB](glossary.html#nlb).

Критерии выбора реализации приведены в разделе [«Балансировка входящего трафика»](admin/configuration/network/ingress/).
