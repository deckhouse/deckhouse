---
title: Канал уведомлений
description: Канал доставки, по которому система мониторинга Deckhouse Platform отправляет алерты получателям, например Telegram, электронная почта или вебхук.
lang: ru
---

Каналы уведомлений, политики уведомлений и отключение уведомлений настраиваются в модуле [`observability`](/modules/observability/). [Алерты](glossary.html#alert) доставляет Alertmanager, который в Deckhouse Platform поддерживает мультитенантность: алерты разделяются на системные (кластерные) и проектные, а каналы и политики доставки настраиваются отдельно в каждом [проекте](glossary.html#project).

Подробнее в разделе [«Модуль observability»](architecture/observability/observability.html).
