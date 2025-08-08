---
title: Configuring notifications on new releases
permalink: en/admin/configuration/update/notifications.html
---

Deckhouse Kubernetes Platform (DKP) generates [alerts](#alerts-in-the-monitoring-system) in the monitoring system
and can automatically send notifications about upcoming minor updates to external systems.
This helps you plan updates and prepare for them in advance.

Conditions for sending notifications to external systems:

- DKP is operating in [automatic update mode](configuration.html#automatic-update-mode).
- A minor version update is planned (notifications are not sent for patch versions).
- A webhook for notifications is configured.

## Alerts in the monitoring system

If an update requires making changes to the cluster (for example, updating the Kubernetes or OS version),
DKP generates designated alerts.
These alerts include:

- **D8NodeHasDeprecatedOSVersion**: Nodes with an unsupported OS version have been detected in the cluster.
- **HelmReleasesHasResourcesWithDeprecatedVersions**: Some Helm releases use deprecated resources.
- **KubernetesVersionEndOfLife**: The installed Kubernetes version is no longer supported.

If any of these alerts appear, make sure to resolve them before updating.
This helps avoid disruptions and ensures the cluster remains stable after the update.

## Configuring notifications

In the `Auto` update mode, you can [configure](/modules/deckhouse/configuration.html#parameters-update-notification) a webhook call to receive a notification about an upcoming minor DKP version update.

Additionally, notifications are generated not only for Deckhouse updates but also for updates of any modules, including individual ones.  
In some cases, the system may initiate multiple notifications simultaneously (10–20 notifications) at approximately 15-second intervals.

{% alert %}
Notifications are available only in the `Auto` update mode; they are not generated in the `Manual` mode.
{% endalert %}

{% alert %}
Specifying a webhook is optional: if the `update.notification.webhook` parameter is not set but the `update.notification.minimalNotificationTime` parameter is specified, the update will still be postponed for the defined duration. In this case, the appearance of the [DeckhouseRelease](../../../reference/api/cr.html#deckhouserelease) resource in the cluster, named after the new version, can be considered the notification.
{% endalert %}

After a new minor DKP version appears in the selected update channel but before it is applied in the cluster, a [POST request](/modules/deckhouse/configuration.html#parameters-update-notification-webhook) will be sent to the configured webhook address.

The [minimalNotificationTime](/modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter allows postponing the update installation for a defined period, providing time to react to the notification while respecting update windows.  
If the webhook is unavailable, each failed attempt to send the notification will postpone the update by the same amount, which may lead to the update being indefinitely deferred.

### Supported notifications parameters

- `update.notification.webhook`: URL for sending notifications.
  A POST request with update information is sent as soon as a new minor version appears on the selected release channel,
  but before the update is installed in the cluster.
- `update.notification.auth`: Authentication parameters for calling the webhook.
  If this parameter is not set, authentication is not used.
  - `update.notification.auth.basic`: Basic authentication.
    The username and password are passed in the `Authorization` header as `Basic <base64(username:password)>`.
    - `update.notification.auth.basic.username`: Username.
    - `update.notification.auth.basic.password`: Password.
  - `update.notification.auth.bearerToken`: Token-based authentication.
    The token is passed in the `Authorization` header as `Bearer <token>`.
- `update.notification.minimalNotificationTime`: Minimum interval between the appearance of a new minor version
  on the selected release channel and the start of the update.
  Specified in hours and minutes: `30m`, `1h`, `2h30m`, `24h`.
  If an [update window](configuration.html#update-windows) is configured,
  the update will be applied only after the `minimalNotificationTime` has elapsed and within the defined window.
- `update.notification.tlsSkipVerify`: Disables TLS certificate verification when calling the webhook
  (for example, if a self-signed certificate is used).
  Set to `false` by default.

Example `update.notification` configuration using basic authentication:

```yaml
update:
  notification:
    webhook: https://release-webhook.mydomain.com
    minimalNotificationTime: 4h
    auth:
      basic:
        username: myusername
        password: mypassword
    tlsSkipVerify: true
```

Example `update.notification` configuration  without authentication:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

## Notification format

When the conditions for sending notifications are met,
DKP sends a POST request to the specified webhook with the header `Content-Type: application/json`.

Example request body:

```json
{
  "version": "1.68",
  "requirements":  {"k8s": "1.29.0"},
  "changelogLink": "https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.68.md",
  "applyTime": "2025-02-01T14:30:00Z00:00",
  "message": "New Deckhouse Release 1.68 is available. Release will be applied at: Wednesday, 05-Feb-25 14:30:00 UTC"
}
```

Field descriptions:

- `version`: Minor version number (a string).
- `requirements`: Object with requirements for the new version (for example, the minimum Kubernetes version).
- `changelogLink`: Link to the changelog describing changes in the new minor version.
- `applyTime`: Scheduled update date and time in RFC3339 format with the configured update windows considered.
- `message`: A short text notification about the available minor version and its scheduled installation time.
