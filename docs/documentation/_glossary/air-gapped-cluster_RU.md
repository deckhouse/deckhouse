---
title: Изолированный кластер
description: Кластер без доступа к внешнему хранилищу образов контейнеров. Единственный источник образов для него — внутрикластерный кеш.
lang: ru
---

В [режиме управления registry](glossary.html#registry-mode) `Managed` кластер считается изолированным, если не задан параметр `primary.upstream`. [Внутрикластерный кеш](glossary.html#in-cluster-cache) при этом наполняется командой `d8 mirror push` и выполняет роль [хранилища образов компонентов Deckhouse Platform](glossary.html#component-registry).

Подробнее в разделе [«Управление хранилищем образов в кластерах, полностью управляемых DP»](admin/configuration/registry/managing-interaction.html).
