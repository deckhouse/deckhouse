---
title: "Platform update"
permalink: en/virtualization-platform/documentation/admin/update/update.html
---

## Platform update configuration

The platform update is configured in the ModuleConfig resource [`deckhouse`](../../../reference/cr/moduleconfig.html).

To view the current configuration of update settings, use the following command:

```shell
d8 k get mc deckhouse -oyaml
```

Example output:

```yaml
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

## Update mode configuration

The platform supports three update modes:

- **Automatic + no update windows set.** The cluster updates immediately after a new version is available on the corresponding [update channel](http://deckhouse.io/products/virtualization-platform/documentation/admin/release-channels.html).
- **Automatic + update windows set.** The cluster updates during the next available update window after a new version is available on the update channel.
- **Manual mode.** Updates require [manual actions](./manual-update-mode.html).

Example configuration snippet for enabling automatic platform updates:

```yaml
update:
  mode: Auto
```

Example configuration snippet for enabling automatic platform updates with update windows:

```yaml
update:
  mode: Auto
  windows:
    - from: "8:00"
      to: "15:00"
      days:
        - Tue
        - Sat
```

Example configuration snippet for enabling manual platform update mode:

```yaml
update:
  mode: Manual
```

## Release channels

The platform uses [five release channels](http://deckhouse.io/products/virtualization-platform/documentation/admin/release-channels.html) designed for various environments. Platform components can be updated either automatically or with manual confirmation as updates are released in the respective channels.

Information about versions available in release channels can be found at [https://releases.deckhouse.io/](https://releases.deckhouse.io/).

To switch to a different release channel, set the `.spec.settings.releaseChannel` parameter in the `deckhouse` module configuration.

Example configuration for the `deckhouse` module with the release channel set to `Stable`:

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

- When switching the update channel to a **more stable** one (e.g., from `Alpha` to `EarlyAccess`), Deckhouse performs the following actions:
  - Downloads release data (in this example, from the `EarlyAccess` channel) and compares it with existing `DeckhouseRelease` resources in the cluster:
    - Later releases that have not yet been applied (status `Pending`) are deleted.
    - If later releases are already applied (status `Deployed`), the release change does not occur. In this case, the platform remains on the current release until a newer release appears in the `EarlyAccess` update channel.
- When switching the update channel to a **less stable** one (e.g., from `EarlyAccess` to `Alpha`):
  - Deckhouse downloads release data (in this example, from the `Alpha` channel) and compares it with existing `DeckhouseRelease` resources in the cluster.
  - The platform then updates according to the configured update parameters.

To view the list of platform releases, use the following commands:

```shell
d8 k get deckhouserelease
d8 k get modulereleases
```

{% offtopic title="Scheme for using the releaseChannel parameter during installation and Platform operation" %}
![Scheme for using the releaseChannel parameter during installation and Platform operation](/images/common/deckhouse-update-process.png)
{% endofftopic %}

To disable the platform update mechanism, remove the `.spec.settings.releaseChannel` parameter from the `deckhouse` module configuration. In this case, the platform does not check for updates, and patch-release updates are not performed.

{% alert level="danger" %}
Disabling automatic updates is highly discouraged. This will block updates to patch releases, which may include critical vulnerability and bug fixes.
{% endalert %}

## Immediate Update Application

To apply an update immediately, set the annotation `release.deckhouse.io/apply-now: "true"` on the corresponding [DeckhouseRelease](../../../reference/cr/deckhouserelease.html) resource.

{% alert level="info" %}
In this case, update windows, [canary-release](../../../reference/cr/deckhouserelease.html#deckhouserelease-v1alpha1-spec-applyafter) settings, and the [manual cluster update mode](../../reference/cr.html#parameters-update-disruptionapprovalmode) will be ignored. The update will be applied immediately after setting the annotation.
{% endalert %}

Example command to set the annotation for bypassing update windows for version `v1.56.2`:

```shell
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Example of a resource with the annotation to bypass update windows:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
```
