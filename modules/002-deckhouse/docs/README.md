---
title: "The deckhouse module"
search: releaseChannel, release channel stabilization, auto-switching the release channel
---

In Deckhouse, this module sets up:
- **[The logging level](configuration.html#parameters-loglevel)**;
- **[The set of modules](configuration.html#parameters-bundle) enabled by default**;

  Usually, the `Default` set is used (it is suitable for most cases).

  Regardless of the set of modules enabled by default, any module can be explicitly enabled or disabled in the Deckhouse configuration (learn more [about enabling and disabling a module](/products/kubernetes-platform/documentation/v1/admin/configuration/#enabling-and-disabling-the-module)).
- **[The release channel](configuration.html#parameters-releasechannel)**;

  Deckhouse has a built-in mechanism for automatic updates. This mechanism uses [5 release channels](/products/kubernetes-platform/documentation/v1/reference/release-channels.html) with various stability and frequency of releases. Learn more about [how the automatic update mechanism works](/products/kubernetes-platform/documentation/v1/architecture/updating.html) and how you can [set the desired release channel](/products/kubernetes-platform/documentation/v1/admin/configuration/update/configuration.html)
- **[The update mode](configuration.html#parameters-update-mode)** and **[update windows](configuration.html#parameters-update-windows)**;

  Deckhouse supports **manual** and **automatic** update modes.

  In the manual upgrade mode, only critical fixes (patch releases) are automatically applied, and upgrading to a more current Deckhouse release requires [manual confirmation](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease-v1alpha1-approved).

  In the automatic update mode, Deckhouse switches to a newer release as soon as it is available in the corresponding release channel unless [update windows](configuration.html#parameters-update-windows) are **configured** for the cluster. If update windows are **configured** for the cluster, Deckhouse will upgrade to a newer release during the next available update window.

- **Service for validating Custom Resources**.

  The validation service prevents creating Custom Resources with invalid values or adding such values to the existing Custom Resources. Note that it only tracks Custom Resources managed by Deckhouse modules.

## Deckhouse releases update

### Get Deckhouse releases status

You can get Deckhouse releases list via the command `d8 k get deckhousereleases`. By default, a cluster keeps the last 10 Superseded releases and all deployed/pending releases.

Every release can have one of the following statuses:

* `Pending` - release is waiting to be deployed: waiting for update window, canary deployment, etc. You can see the detailed status via the `d8 k describe deckhouserelease $name` command.
* `Deployed` - release is applied. It means that the image tag of the Deckhouse Pod was changed, but the update process of all components
is going asynchronously and could not have been finished yet.
* `Superseded` - release is outdated and not used anymore.
* `Suspended` - release was suspended (for example, due to an error found). A release can have this status if it was suspended before being deployed in the cluster.

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

  In this case, Deckhouse holds on a current version and able to receive updates. But to apply the update, a [manual action](usage.html#manual-update-confirmation) will need to be performed. This applies to both patch versions and minor versions.
  
  To enable manual update mode, you need to set the parameter [settings.update.mode](configuration.html#parameters-update-mode) in ModuleConfig `deckhouse` to `Manual`:
  
  ```shell
  d8 k patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"Manual"}}}}'
  ```

- Set the automatic update mode for patch versions.

  In this case, Deckhouse holds on a current version and will automatically update to patch versions of the current release (taking into account the update windows). To apply a minor version update, a [manual action](usage.html#manual-update-confirmation) will need to be performed.
  
  For example: the current version of DKP is `v1.70.1`, after setting the automatic update mode for patch versions, Deckhouse can be updated to version `v1.70.2`, but will not update to version `v1.71.*` or higher.

  To set the automatic update mode for patch versions, you need to set the parameter [settings.update.mode](configuration.html#parameters-update-mode) to `AutoPatch` in the ModuleConfig `deckhouse`:

  ```shell
  d8 k patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"AutoPatch"}}}}'
  ```

- Set a specified image tag for Deployment `deckhouse` and remove [releaseChannel](configuration.html#parameters-releasechannel) parameter from `deckhouse` module configuration.

  In that case, DKP holds on a current version, and no information about new available versions in the cluster (DeckhouseRelease objects) will be received.

  An example of installing version `v1.66.3` for DKP EE and removing the `releaseChannel` parameter from the configuration of the `deckhouse` module:
  
  ```shell
  d8 k -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- kubectl set image deployment/deckhouse deckhouse=registry.deckhouse.io/deckhouse/ee:v1.66.3
  d8 k patch mc deckhouse --type=json -p='[{"op": "remove", "path": "/spec/settings/releaseChannel"}]'
  ```

## Priority Classes

This module creates a set of [priority classes](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass) and assigns them to components installed by Deckhouse and applications in the cluster.

Priority classes are utilized by the scheduler to determine a Pod's priority based on the class it belongs to during scheduling.

For example, when deploying Pods with `priorityClassName: production-low`, if the cluster lacks sufficient resources, Kubernetes will start evicting Pods with the lowest priority to accommodate the `production-low` Pods.
That is, Kubernetes will first evict all Pods with `priorityClassName: develop` Pods, then proceed to `cluster-low` Pods, and so on.

When setting the priority class, it is crucial to understand the type of application and the environment it operates in. Assigning any priority class does not lower a Pod's priority because Pods without a specified priority class are considered to have the lowest priority.

{% alert level="warning" %}
You cannot use the following priority classes: `system-node-critical`, `system-cluster-critical`, `cluster-medium`, `cluster-low`.
{% endalert %}

Below is the list of priority classes set by the module (sorted by the priority from highest to lowest):

| Priority class            | Description                                                                                                                                                                                                                                                                                                                                                           | Value      |
|---------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| `system-node-critical`    | Cluster components that must be present on the node. This priority class fully protects components against [eviction by kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).<br>Examples: `node-exporter`, `csi`, and others.                                                                                                              | 2000001000 |
| `system-cluster-critical` | Cluster components that are critical to its correct operation. This PriorityClass is mandatory for MutatingWebhooks and Extension API servers. It also fully protects components against [eviction by kubelet](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).<br>Examples: `kube-dns`, `kube-proxy`, `cni-flannel`, `cni-cilium`, and others. | 2000000000 |
| `production-high`         | Stateful applications in the production environment. Their unavailability leads to service downtime or data loss.<br>Examples: `PostgreSQL`, `Memcached`, `Redis`, `MongoDB`, and others.                                                                                                                                                                             | 9000       |
| `cluster-medium`          | Cluster components responsible for monitoring (alerts, diagnostic tools) and autoscaling. Without monitoring, assessing the scale of incidents is impossible; without autoscaling, applications cannot receive necessary resources.<br>Examples: `deckhouse`, `node-local-dns`, `grafana`, `upmeter`, and others.                                                     | 7000       |
| `production-medium`       | Main stateless applications in the production environment that are responsible for operating the service for end-users.                                                                                                                                                                                                                                               | 6000       |
| `deployment-machinery`    | Cluster components that are responsible for deploying/building.                                                                                                                                                                                                                                                                                                       | 5000       |
| `production-low`          | Non-critical, secondary applications in the production environment (crons, admin dashboards, batch processing). For important batch or cron jobs, consider assigning them the `production-medium` priority.                                                                                                                                                           | 4000       |
| `staging`                 | Staging environments for applications.                                                                                                                                                                                                                                                                                                                                | 3000       |
| `cluster-low`             | Cluster components that are desirable but not essential for proper cluster operation. <br>Examples: `dashboard`, `cert-manager`, `prometheus`, and others.                                                                                                                                                                                                            | 2000       |
| `develop` (default)       | Develop environments for applications. The default class for a component (if other priority classes aren't set).                                                                                                                                                                                                                                                      | 1000       |
| `standby`                 | This class is not intended for applications. It is used for system purposes (reserving nodes).                                                                                                                                                                                                                                                                        | -1         |
