---
title: Бандл bashible
description: Набор скриптов bashible. Используются четыре бандла, скрипты установки bashible, скрипты бутстрапа первого узла, скрипты настройки узла для облачного провайдера и основные скрипты. Отдельный скрипт бандла называется шагом (степом).
lang: ru
---

Скрипты [bashible](glossary.html#bashible) — это Go-шаблоны, поэтому бандл рендерится для конкретной [группы узлов](glossary.html#node-group), а каждый шаг должен корректно выполняться при повторном запуске после ошибки. Отрендеренные бандлы отдаёт [bashible-api-server](glossary.html#bashible-api-server). Шаги охватывают основные этапы настройки узла: доступ для [NodeUser](/modules/node-manager/cr.html#nodeuser), CA-сертификаты, бинарные файлы в `/opt/deckhouse/bin`, пакеты, containerd, [kubernetes-api-proxy](glossary.html#kubernetes-api-proxy), kubelet и саму службу bashible.

Подробнее в разделе [«Bashible»](architecture/cluster-and-infrastructure/node-management/bashible.html).
