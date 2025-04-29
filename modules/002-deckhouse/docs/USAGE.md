---
title: "The deckhouse module: usage"
---

## Setting up the update mode

You can manage DKP updates in the following ways:
- Using the [settings.update](configuration.html#parameters-update) ModuleConfig `deckhouse` parameter;
- Using the [disruptions](../node-manager/cr.html#nodegroup-v1-spec-disruptions) NodeGroup parameters section.

### Update windows configuration

You can configure the time windows when Deckhouse will automatically install updates in the following ways:
- in the [update.windows](configuration.html#parameters-update-windows) parameter of the `deckhouse` ModuleConfig for overall update management;
- in the [disruptions.automatic.windows](../node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) and [disruptions.rollingUpdate.windows](../node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) parameters of NodeGroup, for managing disruptive updates.

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

<div id="manual-disruption-update-confirmation"></div>

### Manual update confirmation

Manual confirmation of Deckhouse version updates is provided in the following cases:
- The Deckhouse update confirmation mode is enabled.

  This means that the parameter [settings.update.mode](configuration.html#parameters-update-mode) in the ModuleConfig `deckhouse` is set to `Manual` (confirmation for both patch and minor versions of Deckhouse) or `AutoPatch` (confirmation for the minor version of Deckhouse).
  
  To confirm the update, it is necessary to execute the following command, specifying the required version of Deckhouse:

  ```shell
  kubectl patch DeckhouseRelease v1.66.2 --type=merge -p='{"approved": true}'
  ```

- If automatic application of disruptive updates is disabled for a node group.

  This means that the corresponding NodeGroup has the parameter [spec.disruptions.approvalMode](../node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) set to `Manual`.

  For updating **each** node in such a group, the node must have `update.node.deckhouse.io/disruption-approved=` annotation.
  
  Example:

  ```shell
  kubectl annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
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
