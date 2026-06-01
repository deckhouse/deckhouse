---
title: Подсистема Delivery
permalink: ru/architecture/delivery/
lang: ru
search: delivery
description: Архитектура подсистемы Delivery в Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описывается архитектура подсистемы Delivery в Deckhouse Kubernetes Platform (DKP).

В подсистему Delivery входят следующие модули:

* [`pod-reloader`](/modules/pod-reloader/) — предоставляет возможность автоматически  перезапустить workload при изменении ConfigMap или Secret;
* [`operator-argo`](/modules/operator-argo/) — управляет инсталляциями ArgoCD в DKP.
