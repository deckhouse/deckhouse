---
title: Dashboard
description: A collection of charts and tables with data about the state of the cluster and the applications running in it. Deckhouse Platform ships preinstalled dashboards and lets users create custom ones.
lang: en
---

Dashboards show CPU usage, memory consumption, disk and network activity, and the status of pods, controllers, nodes and [namespaces](glossary.html#namespace). They are available in the [Deckhouse web UI](glossary.html#web-ui) under "Monitoring" → "Dashboards".

Preinstalled dashboards are shipped with Deckhouse Platform and cannot be edited. Custom dashboards are created with an [ObservabilityDashboard](/modules/observability/cr.html#observabilitydashboard) object of the [`observability`](/modules/observability/) module at the namespace level with access control, or with a [GrafanaDashboardDefinition](/modules/prometheus/cr.html#grafanadashboarddefinition) object at the cluster level, which is a legacy approach.

For details, see [Monitoring dashboards](user/monitoring/dashboards.html).
