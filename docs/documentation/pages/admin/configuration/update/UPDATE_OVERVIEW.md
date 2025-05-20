---
title: Overview
permalink: en/admin/configuration/update/
---

Deckhouse Kubernetes Platform (DKP) supports a flexible update mechanism
that helps balance the speed of receiving new features with cluster stability.

A brief overview of the key features:

- [Choosing an update mode](configuration.html#update-modes).
  DKP supports three update modes, allowing you to manage not only manual and fully automatic updates,
  but also the application of patch versions.

- [Configuring update windows](configuration.html#update-windows).
  You can specify the days of the week and time periods
  during which the platform will perform automatic updates according to the selected update mode.

- [Flexible switching between release channels](../../../architecture/updating.html#release-channels).
  Switching between release channels does not result in a version downgrade.
  You can switch to a more stable channel or one where new features are delivered earlier.

- [Receiving release notifications](notifications.html#configuring-notifications).
  DKP allows you to configure a webhook to receive automatic notifications when a new version becomes available.

- [Receiving the changelog](../../../architecture/updating.html#retrieving-the-changelog).
  Each DKP release includes a changelog that is available both in the cluster and in the release notification.

- [Dependency awareness during updates](../../../architecture/updating.html#checking-dependencies-before-update).
  DKP takes dependencies specified in the release version into account when updating.
  This prevents updates when there are version conflicts between cluster components or resources.

Thanks to these features, you can update DKP safely and in a timely manner, avoiding downtime and compatibility issues.
