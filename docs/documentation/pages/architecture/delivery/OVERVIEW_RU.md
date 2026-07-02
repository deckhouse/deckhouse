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

- [`operator-argo`](/modules/operator-argo/) — управляет инсталляциями ArgoCD в DKP,
- [`operator-helm`](/modules/operator-helm/) — обеспечивает декларативное управление развертывания Helm-чартов.
