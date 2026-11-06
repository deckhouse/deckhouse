---
title: Модуль из источника
description: Модуль, который Deckhouse Platform получает из источника модулей, а не из собственной поставки. Его релизный цикл не привязан к релизному циклу платформы.
lang: ru
---

Список доступных модулей определяют [источники модулей](glossary.html#module-source), которые отслеживает Deckhouse Platform. Такой модуль обновляется по собственной [политике обновления](glossary.html#module-update-policy): у него свои [каналы обновлений](glossary.html#release-channel), [стадия жизненного цикла](glossary.html#module-lifecycle-stage) и [зависимости](glossary.html#module-dependencies). Алерты модулей из источника не входят в справочник алертов платформы.

Подробнее в разделе [«Обновление DP в закрытом окружении»](guides/airgapped-update.html).
