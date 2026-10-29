---
title: Режим обновления
description: Настройка, которая определяет, как Deckhouse Platform применяет новые версии.
lang: ru
---

В режиме `Auto` автоматически применяются и патч-версии, и минорные версии, в режиме `AutoPatch` — только патч-версии в рамках текущей минорной версии. В режиме `Manual` версия применяется только после того, как администратор подтвердит соответствующий [DeckhouseRelease](reference/api/cr.html#deckhouserelease).

Режим задаётся в параметре `settings.update.mode` [модуля](glossary.html#module) [`deckhouse`](/modules/deckhouse/). В автоматических режимах обновления применяются с учётом [окон обновлений](glossary.html#update-window).

Подробнее в разделе [«Настройка обновлений»](admin/configuration/update/configuration.html).
