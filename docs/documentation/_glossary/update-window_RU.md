---
title: Окно обновлений
description: Интервал времени, в который Deckhouse Platform может устанавливать обновления автоматически. Вне окон новые версии не устанавливаются.
lang: ru
---

Окна обновлений задаются в параметре `update.windows` [модуля](glossary.html#module) [`deckhouse`](/modules/deckhouse/). Если окна не настроены, обновление применяется сразу после появления новой версии на [канале обновлений](glossary.html#release-channel).

Для [disruptive-обновлений](glossary.html#disruptive-update) узлов есть отдельные окна в параметрах [NodeGroup](/modules/node-manager/cr.html#nodegroup) `disruptions.automatic.windows` и `disruptions.rollingUpdate.windows`.

Подробнее в разделе [«Настройка обновлений»](admin/configuration/update/configuration.html).
