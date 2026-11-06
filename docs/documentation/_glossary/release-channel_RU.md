---
title: Канал обновлений
description: Один из пяти каналов, от Alpha до Rock Solid, по которым каждая новая версия Deckhouse Platform распространяется постепенно. Кластер получает версии, опубликованные на выбранном канале.
lang: ru
---

Каждая новая версия сначала публикуется на канале Alpha, затем переходит на Beta, Early Access, Stable и Rock Solid. На Early Access версия появляется не раньше чем через 7 дней после выпуска, на Stable — через 14 дней, на Rock Solid — через 30 дней.

Канал кластера задаётся в параметре `settings.releaseChannel` [модуля](glossary.html#module) [`deckhouse`](/modules/deckhouse/). Как применяется новая версия, зависит от [режима обновления](glossary.html#update-mode) и [окон обновлений](glossary.html#update-window).

Подробнее в разделе [«Обновление»](architecture/updating.html).
