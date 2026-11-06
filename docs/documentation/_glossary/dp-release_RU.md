---
title: Релиз Deckhouse Platform
description: Любая опубликованная версия Deckhouse Platform. Релизы распространяются по каналам обновлений с заданными задержками и бывают двух видов — патч-версия и минорная версия.
lang: ru
---

[Патч-версия](glossary.html#patch-version) содержит исправления ошибок, [минорная версия](glossary.html#minor-version) — новые функции. Каждый новый релиз сначала появляется на [канале обновлений](glossary.html#release-channel) Alpha и затем переходит на более стабильные каналы. В кластере релиз представлен объектом [DeckhouseRelease](reference/api/cr.html#deckhouserelease), который применяется согласно [режиму обновления](glossary.html#update-mode) и [окнам обновлений](glossary.html#update-window). Минорные релизы нельзя пропускать, потому что они содержат миграции, которые должны применяться по порядку.

Подробнее в разделе [«Обновление»](architecture/updating.html).
