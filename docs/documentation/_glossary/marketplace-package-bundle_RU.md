---
title: Bundle пакета
description: Основной образ версии пакета в хранилище образов. Содержит манифест пакета, дайджесты образов, схемы OpenAPI, Helm-шаблоны, бинарный файл хуков, документацию и changelog.
lang: ru
---

Bundle публикуется как `<REPOSITORY>/<PACKAGE_NAME>:<PACKAGE_VERSION>` рядом с образами контейнеров приложения (на них ссылаются по дайджесту), образом метаданных версии и образами [каналов обновлений](glossary.html#release-channel). Deckhouse Platform скачивает bundle нужной версии первым шагом установки [экземпляра приложения](glossary.html#application-instance), затем читает `package.yaml`, схемы и `images_digests.json` и обнаруживает хуки.

Подробнее в разделе [«Разработка приложений»](architecture/marketplace/application-development.html).
