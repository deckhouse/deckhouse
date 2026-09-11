---
title: Подсистема Delivery
permalink: ru/architecture/delivery/
lang: ru
search: delivery
description: Архитектура подсистемы Delivery в Deckhouse Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описывается архитектура подсистемы Delivery в Deckhouse Platform (DP).

В подсистему Delivery входят следующие модули:

- [`operator-argo`](/modules/operator-argo/) — управляет инсталляциями ArgoCD в DP;
- [`operator-helm`](/modules/operator-helm/) — обеспечивает декларативное управление развертыванием Helm-чартов.
