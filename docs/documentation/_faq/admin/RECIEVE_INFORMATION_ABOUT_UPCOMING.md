---
title: How can I receive information about upcoming updates in advance?
subsystems:
  - deckhouse
lang: en
---

You can get information about upcoming minor DKP version updates on the release channel in one of the following ways:

- Enable [manual update mode](configuration.html#manual-update-approval).
  A new [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource and the [`DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval) alert will appear when a new version is available.
- Enable [automatic update mode](configuration.html#automatic-update-mode) and set a delay using the [`minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter.
  A new [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) resource will appear when a new version is available.
  If you also set a webhook URL in the [`update.notification.webhook`](/modules/deckhouse/configuration.html#parameters-update-notification-webhook) parameter,
  a notification will be sent about the upcoming update.
