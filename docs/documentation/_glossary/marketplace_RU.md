---
title: Marketplace
description: Система управления пакетами Deckhouse Platform. Администраторы подключают репозитории пакетов, пользователи устанавливают приложения из пакетов в своих неймспейсах.
lang: ru
---

Администратор кластера подключает репозиторий пакетов с помощью [PackageRepository](reference/api/cr.html#packagerepository), следит за операциями сканирования, в ходе которых Marketplace обнаруживает доступные пакеты, и выдаёт пользователям права на просмотр версий пакетов и создание объектов Application. Установленное приложение — объект [Application](reference/api/cr.html#application) в [неймспейсе](glossary.html#namespace) пользователя.

Marketplace доступен в Deckhouse Platform начиная с версии 1.76.

Подробнее в разделе [«Marketplace»](admin/configuration/marketplace/).
