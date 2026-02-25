---
title: How can I know when a new DKP version is available for the cluster?
subsystems:
  - deckhouse
lang: en
---

As soon as a new version appears on the configured release channel:

- The [`DeckhouseReleaseIsWaitingManualApproval`](../../../reference/alerts.html#monitoring-deckhouse-deckhousereleaseiswaitingmanualapproval) alert will appear if the cluster is in [manual update mode](configuration.html#manual-update-approval).
- A new [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) custom resource will be created.
  To see the list of releases, run `d8 k get deckhousereleases`.
  If the new version is in `Pending` state, it means it hasn’t been installed yet. Possible reasons:
  - [Manual update mode](configuration.html#manual-update-approval) is enabled.
  - Automatic update mode is enabled and [update windows](configuration.html#update-windows) are scheduled, but the window hasn’t started yet.
  - Automatic update mode is enabled and update windows are not scheduled,
    but the update is delayed by a random period to reduce load on the container image registry.
    The `status.message` field of the DeckhouseRelease resource will show a corresponding message.
  - The [`update.notification.minimalNotificationTime`](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter is set, and the delay period hasn’t elapsed.
