---
title: Архив образов Deckhouse Platform
description: Архив образов контейнеров Deckhouse Platform, который создаёт команда d8 mirror pull и загружает в приватное хранилище образов команда d8 mirror push. Так образы платформы доставляются в закрытое окружение.
lang: ru
---

Архив скачивается из публичного хранилища Deckhouse Platform с [лицензионным ключом](glossary.html#license-key), обычно на [bastion-хосте](glossary.html#bastion-host), и переносится в контур любым разрешённым способом, например на съёмном носителе. По его размеру после `d8 mirror push` оценивают объём диска приватного хранилища. Команды `d8 mirror` недоступны в [редакциях](glossary.html#edition) Community Edition и Basic Edition.

Подробнее в разделе [«Установка DP в закрытом окружении»](guides/private-environment.html).
