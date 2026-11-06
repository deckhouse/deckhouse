---
title: Образ канала обновлений
description: Образ release-channel в хранилище образов Deckhouse Platform, тег которого совпадает с названием канала обновлений. Тег указывает на образ конкретной версии Deckhouse Platform и переносится на новый образ при выпуске новой версии.
lang: ru
---

Образ содержит `version.json` с текущей версией [канала обновлений](glossary.html#release-channel) и `changelog.yaml`. Когда версия в образе меняется, Deckhouse Platform создаёт объект [DeckhouseRelease](reference/api/cr.html#deckhouserelease) и начинает обновление. Выкат можно остановить, добавив в образ флаг `suspend`, тогда релиз становится [приостановленным](glossary.html#suspended-release). В [закрытом окружении](glossary.html#private-environment) образ копируется в приватное хранилище вместе с [архивом образов](glossary.html#mirror-image-bundle).

Подробнее в разделе [«Обновление DP в закрытом окружении»](guides/airgapped-update.html).
