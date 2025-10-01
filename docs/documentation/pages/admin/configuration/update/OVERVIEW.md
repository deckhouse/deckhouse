---
title: Overview
permalink: en/admin/configuration/update/
description: "Manage updates for Deckhouse Kubernetes Platform. Safe rolling updates, notifications, and configuration management for platform and cluster components."
---

Deckhouse Kubernetes Platform (DKP) includes a built-in update management mechanism
for both the platform itself and Kubernetes cluster components.
To roll out new versions consistently and safely, DKP uses a five-channel update system
ranging from the newest and unstable (Alpha) to the most thoroughly tested (Rock Solid).
Each new version gradually moves through these channels,
helping identify issues early and ensuring stability in production environments.
For details on each channel, refer to the [Architecture](../../../architecture/updating.html#release-channels) section.

You can configure the update process.
Both automatic and manual modes are supported, as well as update windows and other parameters.
See [Update configuration](configuration.html#update-modes) for details.

An alerting system is used to track the update status and quickly report any issues.
For more information, refer to [Notification settings](notifications.html).

The following features are supported:

- [Release notifications](notifications.html#configuring-notifications): DKP can send release notifications via webhooks.
- [Retrieving the changelog](../../../architecture/updating.html#retrieving-the-changelog):
  Each DKP release includes a changelog, available both in the cluster and as part of the release notification.
- [Checking dependencies before update](../../../architecture/updating.html#checking-dependencies-before-update):
  Checks for component dependencies before proceeding with an update to prevent conflicts.
