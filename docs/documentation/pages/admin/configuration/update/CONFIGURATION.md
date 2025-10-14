---
title: Configuring updates
permalink: en/admin/configuration/update/configuration.html
description: "Configure update settings for Deckhouse Kubernetes Platform including release channels, update modes, and update policies. Automatic and manual update configuration guide."
---

Deckhouse Kubernetes Platform (DKP) supports a flexible update mechanism,
allowing you to select [release channels](../../../architecture/updating.html#release-channels) and configure the update mode.
Release channels help you balance stability with the speed of receiving new features.

The update mode configuration lets you choose between automatic or manual updates
and define update windows during which new versions can be installed.
Together, these features help you avoid updates at inconvenient times and control migration to new releases.

{% alert level="info" %}
Up-to-date information about DKP versions available on different release channels is available at [releases.deckhouse.io](https://releases.deckhouse.io).
{% endalert %}

## Checking the current release channel

To check which release channel is used in your cluster, run the following command:

```shell
d8 k get mc deckhouse -o yaml | grep releaseChannel
```

Example output:

```console
    releaseChannel: Stable
```

## Switching release channels

To switch the release channel, specify the new channel in the [`settings.releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter of the [`deckhouse`](/modules/deckhouse/) module.

Example configuration using the `Stable` channel:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

## Update modes

DKP supports three update modes that determine how new versions are applied:

- **Automatic mode (without update windows)**: The cluster updates as soon as a new version
  appears on the [selected release channel](../../../architecture/updating.html#release-channels).
- **Automatic mode (with update windows)**: The cluster updates during the next available window
  after a new version appears on the release channel.
- **Manual mode**: Updates must be manually approved before they are applied.

### Checking the current update mode

To determine the current update mode used in the cluster,
inspect the configuration of the [`deckhouse`](/modules/deckhouse/) module with the following command:

```shell
d8 k get mc deckhouse -o yaml
```

Example output:

```console
...
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
...
```

### Automatic update mode

Automatic update mode is enabled when the [`releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter is specified in the [`deckhouse`](/modules/deckhouse/) module configuration.
When this condition is met:

1. DKP checks the release channel every minute for new releases.
1. When a new release appears,
   DKP downloads it into the cluster and creates a [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease) custom resource.
1. Once the DeckhouseRelease resource appears in the cluster,
   DKP applies the corresponding update according to the configured update settings.
   By default, the update is performed automatically, at any time.

To view the list and status of all releases in the cluster, run the following command:

```shell
d8 k get deckhousereleases
```

{% alert level="warning" %}
Starting from DKP 1.70 patch releases (for example, an update from version `1.70.1` to version `1.70.2`) are installed taking into account the update windows. Prior to DKP 1.70, patch version updates ignore update windows settings and apply as soon as they are available.
{% endalert %}

#### Release pinning

*Release pinning* refers to fully or partially disabling automatic updates.

There are three ways to restrict automatic updates in Deckhouse:

- Enable manual update approval mode.

  In this mode, DKP will receive updates into the cluster,
  but applying patch and minor versions will require [manual approval](#manual-update-approval).
  
  To enable manual update approval mode,
  set the [`settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) parameter to `Manual` in the [`deckhouse`](/modules/deckhouse/) module configuration using the following command:

  ```shell
  d8 k patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"Manual"}}}}'
  ```

  To approve an update, run the following command, replacing `<DECKHOUSE-VERSION>` with the target DKP version:

  ```shell
  d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Enable automatic updates for patch versions only.

  In this mode, DKP will receive updates into the cluster,
  but applying minor versions will require [manual approval](#manual-update-approval).
  Patch versions within the current minor version will be applied automatically,
  taking update windows into account if they are configured.

  For example, if you have DKP version `v1.70.1` installed,
  after enabling this mode, Deckhouse can automatically update to `v1.70.2`,
  but it will not update to `v1.71.*` without manual approval.

  To enable automatic updates for patch versions only,
  set the [`settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) parameter to `AutoPatch` in the [`deckhouse`](/modules/deckhouse/) module configuration using the following command:

  ```shell
  d8 k patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"AutoPatch"}}}}'
  ```

  To approve a minor version update,
  run the following command, replacing `<DECKHOUSE-VERSION>` with the target DKP version:

  ```shell
  d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Manually set the target DKP version tag for the `deckhouse` Deployment
  and remove the [`releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter from the [`deckhouse`](/modules/deckhouse/) module configuration.

  In this case, DKP will remain at the specified version,
  and no information about newer available versions (DeckhouseRelease objects) will appear in the cluster.
  
  > **Important**. This mode blocks the installation of patch releases,
  > which may include critical security or bug fixes.

  Example of pinning version `v1.66.3` for DKP EE
  and removing the `releaseChannel` parameter from the [`deckhouse`](/modules/deckhouse/) module configuration:

  ```shell
  d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- kubectl set image deployment/deckhouse deckhouse=registry.deckhouse.ru/deckhouse/ee:v1.66.3
  d8 k patch mc deckhouse --type=json -p='[{"op": "remove", "path": "/spec/settings/releaseChannel"}]'
  ```

### Manual update approval

Manual approval of DKP updates is required in the following cases:

- The DKP update confirmation mode is enabled.

  This means the [`settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) parameter of the [`deckhouse`](/modules/deckhouse/) module is set to either
  `Manual` (confirmation required for both patch and minor updates) or
  `AutoPatch` (confirmation required only for minor updates).
  
  To approve an update, run the following command, replacing `<DECKHOUSE-VERSION>` with the target version:

  ```shell
  d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Automatic update approval is disabled for a NodeGroup,
  for updates that might cause temporary downtime of system components.

  This means the [`spec.disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) parameter of the corresponding NodeGroup resource is set to `Manual`.

  To apply the update, set the `update.node.deckhouse.io/disruption-approved=` annotation on each node in the group:

  Example command:

  ```shell
  d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

## Update windows

DKP allows you to define *update windows*, which are specific time intervals during which automatic updates are allowed.
Using update windows ensures that updates won’t be installed at inconvenient times
or during periods of high cluster load.

### Applying updates when update windows are configured

- If update windows are configured, DKP installs new versions only during the specified windows.
- If no update windows are configured,
  the update is applied as soon as a new version appears on the configured release channel.

### Configuring update windows

You can manage DKP update windows in the following ways:

- **To control general updates**, use the [`update.windows`](/modules/deckhouse/configuration.html#parameters-update-windows) parameter in the [`deckhouse`](/modules/deckhouse/) module configuration.
- **To control updates that may lead to short-term downtime of system components**,
  use the [`disruptions.automatic.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-automatic-windows) and [`disruptions.rollingUpdate.windows`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-rollingupdate-windows) parameters in the NodeGroup resource.

#### Configuration examples

- Two daily update windows: from 08:00 to 10:00 and from 20:00 to 22:00 (UTC):

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

- Update windows on Tuesdays and Saturdays from 18:00 to 19:30 (UTC):

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
