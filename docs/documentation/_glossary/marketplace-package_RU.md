---
title: Пакет Marketplace
description: "Абстрактная единица Marketplace: [приложение](reference/api/cr.html#application), то есть пользовательская нагрузка в неймспейсе, или модуль, расширяющий возможности кластера."
lang: ru
---

Пакеты публикуются в хранилищах образов, подключённых к [Marketplace](glossary.html#marketplace) как репозитории пакетов. Для каждой обнаруженной версии пакета Deckhouse Platform хранит её метаданные: описание, [стадию жизненного цикла](glossary.html#module-lifecycle-stage), требования, [подтверждение удаления](glossary.html#deletion-confirmation), changelog и схемы OpenAPI настроек и values.

Подробнее в разделе [«Концепции»](architecture/marketplace/concepts.html).
