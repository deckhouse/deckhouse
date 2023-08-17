---
title: "The deckhouse module: usage"
---

## Usage

Example of module configuration with automatic Deckhouse update on the EarlyAccess release channel:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
```

## Setting up the update mode

Deckhouse will update as soon as a new release will be created if update windows are not set and the update mode is `Auto`.

Patch versions (e.g. updates from `1.26.1` to `1.26.2`) are installed without confirmation and without taking into account update windows.

> You can also configure node disruption update windows by using the [`disruptions.automatic.windows`](../040-node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) parameter of the `NodeGroup` custom resource.

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

You can also set up updates on certain days, for example, on Tuesdays and Saturdays from 18:00 to 19:30 (UTC):

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

Manual update confirmation can be enabled by using the [update.mode](configuration.html#parameters-update-mode) parameter of the module. In this mode, confirming every **minor** Deckhouse update will be necessary. The patch versions of Deckhouse will be applied automatically in this mode without any confirmation.

Module configuration example (enabling the Stable release channel with manual update mode):

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

The `approved` field must be set to `true` in the corresponding custom resource [`DeckhouseRelease`](cr.html#deckhouserelease) to confirm the update.

Example of confirmation of the update to the version `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

### Manual disruption update confirmation

{% alert %}
A **disruptive update** can temporarily interrupt the operation of an important cluster component, user application, or related systems. For example, such an update may overwrite a default value or change the behavior of some modules.
{% endalert %}

You can enable manual confirmation of _disruptive updates_ using the [update.disruptionApprovalMode](configuration.html#parameters-update-disruptionapprovalmode) parameter — refer to the configuration example below:

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

In this mode, it will be necessary to confirm each minor disruptive update with the `release.deckhouse.io/disruption-approved=true` annotation on the [`DeckhouseRelease`](cr.html#deckhouserelease) resource. A usual update (not disruptive) will be applied automatically.

An example of confirmation of a potentially dangerous Deckhouse minor update `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```

{% alert level="warning" %}
The [disruptionApprovalMode](configuration.html#parameters-update-disruptionapprovalmode) parameter does not affect the cluster update mode (the [update.mode](configuration.html#parameters-update-mode) parameter). For example, with the following configuration, Deckhouse will be updated automatically according to the update window (on Mondays and Tuesdays from 10 to 13 UTC), but will not be updated on versions that are marked as disruptive:

```yaml
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
      windows:
      - days:
        - Mon
        - Tue
        from: "10:00"
        to: "13:00"
```

{% endalert %}

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

## Collect debug info

Read [the FAQ](faq.html#how-to-collect-debug-info) to learn more about collecting debug information.
