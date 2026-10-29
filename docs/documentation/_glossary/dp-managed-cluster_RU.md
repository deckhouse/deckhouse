---
title: Кластер, полностью управляемый Deckhouse Platform
description: Кластер, в котором Deckhouse Platform управляет и control plane, и узлами. Противоположность — Managed Kubernetes-кластер, в котором control plane управляет облачный провайдер.
lang: ru
---

Deckhouse Platform можно установить и в существующий Managed Kubernetes-кластер, например GKE, EKS или AKS. В этом случае Deckhouse Platform не управляет [control plane](glossary.html#control-plane), а часть возможностей ограничена API облачного провайдера: например, управление узлами и модуль [`registry`](/modules/registry/) доступны только в кластерах, полностью управляемых Deckhouse Platform.

Подробнее в разделе [«Установка платформы»](installing/).
