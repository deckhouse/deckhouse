---
title: Расширенный мониторинг
description: Режим мониторинга модуля [`extended-monitoring`](/modules/extended-monitoring/), который добавляет мониторинг секретов и срока действия TLS-сертификатов, сбор событий кластера в виде метрик, проверку доступности образов в registry и мониторинг объектов в неймспейсах с лейблом.
lang: ru
---

Объекты отслеживаются в неймспейсах с лейблом `extended-monitoring.deckhouse.io/enabled=""`. Доступность образов проверяется для образов, которые используют Deployment, StatefulSet, DaemonSet и CronJob.

Подробнее в разделе [«Мониторинг в Deckhouse Platform»](admin/configuration/monitoring/).
