---
title: Alert notification channel
description: A delivery channel through which the monitoring system of Deckhouse Platform sends alerts to recipients, such as Telegram, email or a webhook.
lang: en
---

Notification channels, notification policies and silences are configured in the [`observability`](/modules/observability/) module. [Alerts](glossary.html#alert) are delivered by Alertmanager, which in Deckhouse Platform supports multitenancy: alerts are divided into system (cluster) and project ones, and channels and delivery policies can be configured separately in every [project](glossary.html#project).

For details, see [Observability module](architecture/observability/observability.html).
