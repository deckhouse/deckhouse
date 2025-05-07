---
title: Configuring updates
permalink: en/admin/configuration/update/configuration.html
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
sudo -i d8 k get mc deckhouse -o yaml | grep releaseChannel
```

Example output:

```console
    releaseChannel: Stable
```

## Switching release channels

To switch the release channel, specify the new channel in the [`settings.releaseChannel`](../../../reference/mc/deckhouse/#parameters-releasechannel) parameter of the `deckhouse` module.

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
inspect the configuration of the `deckhouse` module with the following command:

```shell
sudo -i d8 k get mc deckhouse -o yaml
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

Automatic update mode is enabled when the [`releaseChannel`](../../../reference/mc/deckhouse/#parameters-releasechannel) parameter is specified in the `deckhouse` module configuration.
When this condition is met:

1. DKP checks the release channel every minute for new releases.
1. When a new release appears,
   DKP downloads it into the cluster and creates a [DeckhouseRelease](../../../reference/cr/deckhouserelease) custom resource.
1. Once the DeckhouseRelease resource appears in the cluster,
   DKP applies the corresponding update according to the configured update settings.
   By default, the update is performed automatically, at any time.

To view the list and status of all releases in the cluster, run the following command:

```shell
sudo -i d8 k get deckhousereleases
```

{% alert level="info" %}
Patch updates (for example, from `1.30.1` to `1.30.2`) are installed automatically,
regardless of update mode or update windows.
A new patch release is automatically applied when it appears on the configured release channel.
{% endalert %}

#### Disabling automatic updates

{% alert level="danger" %}
Disabling automatic updates blocks the installation of patch releases,
which may contain critical vulnerability and bug fixes.
{% endalert %}

To completely disable automatic DKP updates, remove the [`releaseChannel`](../../../reference/mc/deckhouse/#parameters-releasechannel) parameter from the `deckhouse` module configuration.

### Manual update approval

Manual approval of DKP updates is required in the following cases:

- The DKP update confirmation mode is enabled.

  This means the [`settings.update.mode`](../../../reference/mc/deckhouse/#parameters-update-mode) parameter of the `deckhouse` module is set to either
  `Manual` (confirmation required for both patch and minor updates) or
  `AutoPatch` (confirmation required only for minor updates).
  
  To approve an update, run the following command, replacing `<DECKHOUSE-VERSION>` with the target version:

  ```shell
  sudo -i d8 k patch DeckhouseRelease <DECKHOUSE-VERSION> --type=merge -p='{"approved": true}'
  ```

- Automatic update approval is disabled for a NodeGroup,
  for updates that might cause temporary downtime of system components.

  This means the [`spec.disruptions.approvalMode`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-approvalmode) parameter of the corresponding NodeGroup resource is set to `Manual`.

  To apply the update, set the `update.node.deckhouse.io/disruption-approved=` annotation on each node in the group:

  Example command:

  ```shell
  sudo -i d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
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

- **To control general updates**, use the `update.windows` parameter in the `deckhouse` module configuration.
- To control updates that may lead to short-term downtime of system components,
  use the [`disruptions.automatic.windows`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-automatic-windows) and [`disruptions.rollingUpdate.windows`](../../../reference/cr/nodegroup/#nodegroup-v1-spec-disruptions-rollingupdate-windows) parameters in the NodeGroup resource.

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
