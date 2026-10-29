---
title: Внутрикластерный кеш образов
description: Хранилище образов контейнеров, развёрнутое на master-узлах кластера, из которого все узлы получают образы компонентов Deckhouse Platform.
lang: ru
---

Кеш включается параметром `storage.cache` в [ModuleConfig](reference/api/cr.html#moduleconfig) [`registry`](/modules/registry/) в [режиме управления registry](glossary.html#registry-mode) `Managed`. Пока кеш наполняется, upstream-хранилище остаётся резервным путём. В [изолированном кластере](glossary.html#air-gapped-cluster) кеш является единственным источником образов и наполняется командой `d8 mirror push`.
