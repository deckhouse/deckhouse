---
title: Минорная версия
description: Релиз Deckhouse Platform с новыми функциями.
lang: ru
---

Минорная версия — один из двух видов [релиза Deckhouse Platform](glossary.html#dp-release), второй вид — [патч-версия](glossary.html#patch-version). Минорные версии устанавливаются последовательно: если кластер отстаёт от [канала обновлений](glossary.html#release-channel) на несколько минорных версий, Deckhouse Platform восстанавливает промежуточные релизы, потому что они содержат миграции, которые должны применяться по порядку. В [режиме обновления](glossary.html#update-mode) `Manual` минорная версия требует подтверждения.

Подробнее в разделе [«Обновление»](architecture/updating.html).
