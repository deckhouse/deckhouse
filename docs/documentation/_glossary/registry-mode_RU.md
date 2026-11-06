---
title: Режим управления registry
description: Режим, в котором модуль registry управляет путями загрузки образов компонентов Deckhouse Platform, Unmanaged или Managed.
lang: ru
---

Режим задаётся параметром `mode` в [ModuleConfig](reference/api/cr.html#moduleconfig) [`registry`](/modules/registry/):

- `Unmanaged` (по умолчанию). Модуль `registry` не управляет путями загрузки: кластер загружает образы из того [хранилища](glossary.html#component-registry), с которым был установлен.
- `Managed`. Модуль `registry` управляет путями загрузки. Параметр `primary.upstream` задаёт upstream-хранилище, а `storage.cache` включает [внутрикластерный кеш образов](glossary.html#in-cluster-cache) на master-узлах. Без `primary.upstream` кластер считается [изолированным](glossary.html#air-gapped-cluster).

Подробнее в разделе [«Управление хранилищем образов в кластерах, полностью управляемых DP»](admin/configuration/registry/managing-interaction.html).
