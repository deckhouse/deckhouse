---
title: "The deckhouse module"
search: releaseChannel, release channel stabilization, auto-switching the release channel
---

In Deckhouse, this module sets up:
- **[The logging level](configuration.html#parameters-loglevel)**;
- **[The set of modules](configuration.html#parameters-bundle) enabled by default**;

  Usually, the `Default` set is used (it is suitable for most cases).

  Regardless of the set of modules enabled by default, any module can be explicitly enabled or disabled in the Deckhouse configuration (learn more [about enabling and disabling a module](../../#enabling-and-disabling-the-module)).
- **[The release channel](configuration.html#parameters-releasechannel)**;

  Deckhouse has a built-in mechanism for automatic updates. This mechanism uses [5 release channels](../../deckhouse-release-channels.html) with various stability and frequency of releases. Learn more about [how the automatic update mechanism works](../../deckhouse-faq.html#how-does-automatic-deckhouse-update-work) and how you can [set the desired release channel](../../deckhouse-faq.html#how-do-i-set-the-desired-release-channel).
- **[The update mode](configuration.html#parameters-update-mode)** and **[update windows](configuration.html#parameters-update-windows)**;

  Deckhouse supports **manual** and **automatic** update modes.

  In the manual upgrade mode, only critical fixes (patch releases) are automatically applied, and upgrading to a more current Deckhouse release requires [manual confirmation](../../cr.html#deckhouserelease-v1alpha1-approved).

  In the automatic update mode, Deckhouse switches to a newer release as soon as it is available in the corresponding release channel unless [update windows](configuration.html#parameters-update-windows) are **configured** for the cluster. If update windows are **configured** for the cluster, Deckhouse will upgrade to a newer release during the next available update window.

- **Service for validating Custom Resources**.

  The validation service prevents creating Custom Resources with invalid values or adding such values to the existing Custom Resources. Note that it only tracks Custom Resources managed by Deckhouse modules.

## Deckhouse releases update

### Get Deckhouse releases status

You can get Deckhouse releases list via the command `kubectl get deckhousereleases`. By default, a cluster keeps the last 10 Superseded releases and all deployed/pending releases.

Every release can have one of the following statuses:
* `Pending` - release is waiting to be deployed: waiting for update window, canary deployment, etc. You can see the detailed status via the `kubectl describe deckhouserelease $name` command.
* `Deployed` - release is applied. It means that the image tag of the Deckhouse Pod was changed, but the update process of all components
is going asynchronously and could not have been finished yet.
* `Superseded` - release is outdated and not used anymore.
* `Suspended` - release was suspended (for ex. it has an error). Can be set only if `suspended` release was not deployed yet.

### Update process

When release status is changed to `Deployed` state, release is updating only a tag of the Deckhouse image.
Deckhouse will start checking and updating process of the all modules, which were changed from the last release.
Duration of an update could be different and connected to cluster size, enabled modules count and settings.
Foe example: if you cluster have a lot of `NodeGroup` resources, it will take some time to update them because these resources are updated one by one
`IngressNginxControllers` also updating one by one.

### Manual release deployment

If you have a [manual update mode](usage.html#manual-update-confirmation) enabled and have a few Pending releases,
you can approve them all at once. In that case Deckhouse will update in series keeping a release order and changing their status during the update.

### Pin a release

To pin a release means fully or partially disable the automatic Deckhouse version update.

There are three options to limit the automatic update of Deckhouse:
- Enable a manual update mode.

  In this case, Deckhouse holds on a current version and able to receive updates. But to apply the update, a [manual action](usage.html#manual-confirmation-of-updates) will need to be performed. This applies to both patch versions and minor versions.
  
  To enable manual update mode, you need to set the parameter [settings.update.mode](configuration.html#parameters-update-mode) in ModuleConfig `deckhouse` to `Manual`:
  
  ```shell
  kubectl patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"Manual"}}}}'
  ```

- Set the automatic update mode for patch versions.

  In this case, Deckhouse holds on a current version and will automatically update to patch versions of the current release. To apply a minor version update, a [manual action](usage.html#manual-confirmation-of-updates) will need to be performed.
  
  For example: the current version of DKP is `v1.65.2`, after setting the automatic update mode for patch versions, Deckhouse can be updated to version `v1.65.6`, but will not update to version `v1.66.*` or higher.

  To set the automatic update mode for patch versions, you need to set the parameter [settings.update.mode](configuration.html#parameters-update-mode) to `AutoPatch` in the ModuleConfig `deckhouse`:

  ```shell
  kubectl patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"AutoPatch"}}}}'
  ```

- Set a specified image tag for Deployment `deckhouse` and remove [releaseChannel](configuration.html#parameters-releasechannel) parameter from `deckhouse` module configuration.

  In that case, DKP holds on a current version, and no information about new available versions in the cluster (DeckhouseRelease objects) will be received.

  An example of installing version `v1.66.3` for DKP EE and removing the `releaseChannel` parameter from the configuration of the `deckhouse` module:
  
  ```shell
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- kubectl set image deployment/deckhouse deckhouse=registry.deckhouse.io/deckhouse/ee:v1.66.3
  kubectl patch mc deckhouse --type=json -p='[{"op": "remove", "path": "/spec/settings/releaseChannel"}]'
  ```
