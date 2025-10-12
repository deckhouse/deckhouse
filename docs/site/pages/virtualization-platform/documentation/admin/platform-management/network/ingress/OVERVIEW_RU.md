---
title: "Обзор"
permalink: ru/virtualization-platform/documentation/admin/platform-management/network/ingress/
lang: ru
---

В этом разделе описываются подходы к балансировке входящего трафика в Deckhouse Virtualization Platform (DVP):

- NLB (Network Load Balancer) — работает на сетевом уровне, маршрутизирует трафик по IP-адресам и портам без анализа содержимого запросов.
- ALB (Application Load Balancer) — действует на прикладном уровне, анализирует HTTP(S)-заголовки, пути и домены. Поддерживает SSL-терминацию и маршрутизацию в зависимости от содержимого запроса.

## Балансировка на сетевом уровне (NLB)

Балансировка NLB может быть организована двумя способами:

- с помощью внешнего балансировщика от облачного провайдера,
- средствами внутреннего балансировщика MetalLB, работающего как в облачных, так и в bare-metal-кластерах.

## Балансировка на прикладном уровне (ALB)

Для балансировки трафика на уровне приложений используется [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) (модуль [`ingress-nginx`](/modules/ingress-nginx/)).
