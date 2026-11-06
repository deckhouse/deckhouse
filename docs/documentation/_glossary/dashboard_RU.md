---
title: Дашборд
description: Набор графиков и таблиц с данными о состоянии кластера и запущенных в нём приложений. Deckhouse Platform поставляет предустановленные дашборды и позволяет создавать пользовательские.
lang: ru
---

Дашборды показывают загрузку CPU, потребление памяти, дисковую и сетевую активность, а также состояние подов, контроллеров, узлов и [неймспейсов](glossary.html#namespace). Они доступны в [веб-интерфейсе Deckhouse](glossary.html#web-ui) в разделе «Мониторинг» → «Дашборды».

Предустановленные дашборды поставляются вместе с Deckhouse Platform и не редактируются. Пользовательские дашборды создаются с помощью объекта [ObservabilityDashboard](/modules/observability/cr.html#observabilitydashboard) модуля [`observability`](/modules/observability/) на уровне неймспейса с управлением доступом или с помощью объекта [GrafanaDashboardDefinition](/modules/prometheus/cr.html#grafanadashboarddefinition) на уровне кластера, этот способ считается устаревшим.

Подробнее в разделе [«Дашборды мониторинга»](user/monitoring/dashboards.html).
