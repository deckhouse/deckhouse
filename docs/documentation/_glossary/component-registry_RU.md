---
title: Хранилище образов компонентов Deckhouse Platform
description: Хранилище образов контейнеров, из которого кластер загружает образы компонентов Deckhouse Platform.
lang: ru
---

Хранилище указывается при развёртывании кластера в параметре [`deckhouse`](/modules/deckhouse/) в [InitConfiguration](reference/api/cr.html#initconfiguration). Управление его настройками в работающем кластере зависит от способа управления кластером: в [кластере под управлением Deckhouse Platform](glossary.html#dp-managed-cluster) используется модуль [`registry`](/modules/registry/), а [режим управления registry](glossary.html#registry-mode) задаётся в его [ModuleConfig](reference/api/cr.html#moduleconfig). В Managed Kubernetes-кластере используется `helper change-registry`.

Подробнее в разделе [«Хранилище образов компонентов DP»](admin/configuration/registry/).
