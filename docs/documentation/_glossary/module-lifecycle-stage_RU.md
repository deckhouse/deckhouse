---
title: Стадия жизненного цикла модуля
description: Стадия зрелости модуля или пакета Marketplace — Experimental, Preview, General Availability или Deprecated. Объявляется в module.yaml или package.yaml и вместе с каналом обновлений определяет стабильность версии.
lang: ru
---

- `Experimental` — функциональность может сильно измениться, совместимость с будущими версиями не гарантируется. Такие модули нельзя включить, пока параметр `allowExperimentalModules` модуля [`deckhouse`](/modules/deckhouse/) не установлен в `true`.
- `Preview` — предварительная версия. Основные возможности сохранятся, совместимость обеспечивается, но может потребоваться миграция.
- `General Availability` — модуль готов к использованию в production-средах.
- `Deprecated` — разработка и поддержка прекращены.

Стадия [пакета Marketplace](glossary.html#marketplace-package) публикуется в метаданных пакета при сканировании репозитория Deckhouse Platform.

Подробнее в разделе [«Версионирование модулей»](architecture/module-development/versioning/).
