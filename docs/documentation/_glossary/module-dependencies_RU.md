---
title: Зависимости модуля
description: Набор условий (требований), которые должны выполняться, чтобы Deckhouse Platform мог запустить модуль. Объявляются в секции requirements файла module.yaml.
lang: ru
---

Модуль может зависеть от версии Deckhouse Platform, версии Kubernetes и версий других [модулей](glossary.html#module). Зависимости от других модулей вместе с [весом модуля](glossary.html#module-weight) влияют на порядок запуска, а зависимость может быть [условной](glossary.html#module-conditional-dependency). [Пакет Marketplace](glossary.html#marketplace-package) объявляет такие же требования в `package.yaml`, включая группы альтернативных и несовместимых модулей.

Подробнее в разделе [«Зависимости модулей Deckhouse Platform»](architecture/module-development/dependencies/).
