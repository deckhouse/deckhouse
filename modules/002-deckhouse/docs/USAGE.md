---
title: "The deckhouse module: usage"
---

## Usage

Below is a simple example of the module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    logLevel: Debug
    bundle: Minimal
    releaseChannel: EarlyAccess
```

You can also configure additional parameters.

## Setting up the update mode

Deckhouse will update as soon as a new release will be created if update windows are not set and the update mode is `Auto`.

Patch versions (e.g. updates from `1.26.1` to `1.26.2`) are installed without confirmation and without taking into account update windows.

{% alert %}
You can also configure node disruption update window in custom resource [NodeGroup](../040-node-manager/cr.html#nodegroup) (the `disruptions.automatic.windows` parameter).
{% endalert %}

### Update windows configuration

You can configure the time when Deckhouse will install updates by using the [update.windows](configuration.html#parameters-update-windows) module configuration parameter.

An example of setting up two daily update windows — from 8 a.m. to 10 a.m. and from 8 p.m. to 10 p.m. (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows: 
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

You can also set up updates on certain days, for example, on Tuesdays and Saturdays from 6 p.m. to 7:30 p.m. (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows: 
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```

### Manual update confirmation

If necessary, it is possible to enable manual confirmation of updates. This can be done as follows:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      mode: Manual
```

In this mode, it will be necessary to confirm each minor Deckhouse updates (excluding patch versions).

Manual confirmation of the update to the version `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

### Manual disruption update confirmation

If necessary, it is possible to enable manual confirmation of disruptive updates (updates that change the default values or behavior). This can be done as follows:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

In this mode, it will be necessary to confirm each minor disruptive update with the `release.deckhouse.io/disruption-approved=true` annotation on the [DeckhouseRelease](../../cr.html#deckhouserelease) resource.

An example of confirmation of a potentially dangerous Deckhouse minor update `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```

### Deckhouse update notification

In the `Auto` update mode, you can [set up](configuration.html#parameters-update-notification) a webhook call, to be notified of an upcoming Deckhouse minor version update.

An example:

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
```

After a new Deckhouse minor version appears on the update channel, a [POST request](configuration.html#parameters-update-notification-webhook) will be executed to the webhook's URL before it is applied in the cluster.

Set the [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime) parameter to have enough time to react to a Deckhouse update notification. In this case, the update will happen after the specified time, considering the update windows.

An example:

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

{% alert %}
If you do not specify the address in the [update.notification.webhook](configuration.html#parameters-update-notification-webhook) parameter, but specify the time in the [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime) parameter, then the release will still be postponed for at least the time specified in the `minimalNotificationTime` parameter. In this case, the notification of the appearance of a new version can be considered the appearance of a [DeckhouseRelease](../../cr.html#deckhouserelease) resource with a name corresponding to the new version.
{% endalert %}

## Collect debug info

Read [the FAQ](faq.html#how-to-collect-debug-info) to learn more about collecting debug information.
