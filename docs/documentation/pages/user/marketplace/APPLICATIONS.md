---
title: Installing and managing applications
permalink: en/user/marketplace/applications.html
description: "Install, update, and delete applications in Deckhouse Platform Marketplace. Browse available package versions, create Application, check status conditions, manage multiple instances, configure workload resources and autoscaling, automatic updates, and version compatibility rules."
lang: en
search: Application, install application, application conditions, installing application, updating application, deleting application, resourceRequests, workload resources, autoscaling, minReplicas, maxReplicas, HorizontalPodAutoscaler, VerticalPodAutoscaler, automatic updates, update windows, spec.update, versionCompatibilityRules, upgrade.from, version compatibility rules
---

This section describes how to browse available [package](../../glossary.html#marketplace-package) versions, as well as how to install, update, and delete applications.

## Browsing available package versions

To list all available package versions, use the following command (you can use `apv` as the short name for ApplicationPackageVersion):

```bash
d8 k get apv
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                           PACKAGE    REPOSITORY    METADATALOADED   USEDBY   AGE
my-registry-redis-v7.2.0       redis      my-registry   True             1        2d
my-registry-redis-v7.3.0       redis      my-registry   True                      5h
my-registry-postgres-v15.0.0   postgres   my-registry   True             2        2d
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

The name of an ApplicationPackageVersion object is formatted as `<REPOSITORY_NAME>-<PACKAGE_NAME>-<PACKAGE_VERSION>`. To also see when the metadata was loaded and the loading error, if any, add `-o wide` to the command.

To filter by package name, use the following command (the example filters versions of the `redis` package):

```bash
d8 k get apv -l packages.deckhouse.io/package=redis
```

To see which package repositories provide a package, use the following command (you can use `ap` as the short name for ApplicationPackage):

```bash
d8 k get ap redis \
  -o jsonpath='{.status.availableRepositories}'
```

{% alert level="info" %}
If you see `MetadataLoaded=True` in the output, it means that the package's OpenAPI schema, description, and requirements were successfully loaded from the repository. A package version with `MetadataLoaded=False` cannot be installed until the metadata is loaded.
{% endalert %}

### Packages available in a project

If the `multitenancy-manager` module is enabled, each project namespace has an [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) named `applicationpackages` that lists the packages available for installation in the project. Unlike ApplicationPackage objects, it can be read with the roles of the namespace.

To view the `applicationpackages` contents, use the following command:

```bash
d8 k get available applicationpackages -n <NAMESPACE> -o yaml
```

Abridged example output:

```console
status:
  grantedResourceKind: ApplicationPackage
  availableCount: 2
  available:
    - name: postgres
      fields:
        availableRepositories:
          - my-registry
        releaseChannels:
          my-registry:
            stable: v15.0.0
    - name: redis
      fields:
        availableRepositories:
          - my-registry
        releaseChannels:
          my-registry:
            alpha: v7.3.0
            stable: v7.2.0
```

The `fields.availableRepositories` field lists the package repositories that provide the package, and `fields.releaseChannels` lists the package versions on the release channels of each repository.

The list of package versions (`d8 k get apv`) also includes the packages that are not available to the project.

An Application of a package that is not available to the project is rejected with the following message:

```text
[multitenancy] Application "<APPLICATION_NAME>" references "<PACKAGE_NAME>" which is not available to project "<PROJECT_NAME>". Ask the cluster administrator to grant it.
```

The cluster administrator selects the packages available for installation in a project with a ClusterResourceGrantPolicy for the `applicationpackages` resource (for details, refer to ["Configuring package access"](../../admin/configuration/marketplace/package-access.html)). By default, every package is available to every project. An application installed before its package became unavailable to the project keeps working.

## Installing an application

To install an application, create an [Application](../../reference/api/cr.html#application) object in the target namespace.

Example manifest for installing Redis from the `redis` package version `v7.2.0` with the `maxmemory` setting:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-cache
  namespace: my-app
spec:
  packageName: redis
  packageVersion: "v7.2.0"
  # Name of the PackageRepository that provides the package.
  packageRepositoryName: my-registry
  settings:
    replicas: 3
    maxmemory: "256mb"
```

{% alert level="info" %}
The `spec.settings` section is validated against the OpenAPI schema defined in the [package](../../glossary.html#marketplace-package). If the schema rejects your settings, the Application is not created. The schema is published in the [`status.packageSchemas.settingsSchema`](../../reference/api/cr.html#applicationpackageversion-v1alpha1-status-packageschemas-settingsschema) field of the package's ApplicationPackageVersion object.
{% endalert %}

### Naming constraints

The Application name (`metadata.name`) must be **at most 24 characters**. An Application with a longer name is rejected on creation. The instance name is a part of the names of all application objects. The objects are named in the format `d8a-<INSTANCE_NAME>-<SUFFIX>`, for example, `d8a-redis-cache-server`, and must fit the Kubernetes name length limits (for details, refer to ["Architecture"](../../architecture/marketplace/concepts.html#naming-constraints)).

## Checking application status

To get a brief status of an application, use the following command:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME>
```

Example output:

```console
NAME          PACKAGE   VERSION   STATE   MESSAGE   AGE
redis-cache   redis     v7.2.0    Ready             5m
```

To also see the repository and the `Installed` and `Ready` conditions, add `-o wide`.

To get the full status including conditions, use the following command:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> -o yaml
```

### Conditions

The application state is described in detail through a set of conditions:

| Condition | Meaning |
|---|---|
| `Installed` | The initial installation is complete: the package is downloaded, the hooks have run, the manifests are applied, and the Deployments and StatefulSets of the application are ready |
| `UpdateInstalled` | The new version is installed: it is downloaded, the hooks have run, and the manifests are applied. The condition appears after the package version is changed for the first time |
| `ConfigurationApplied` | The current configuration is applied: the settings, the hooks, and the manifests |
| `Scaled` | All Deployments and StatefulSets of the application have rolled out and have the desired number of ready replicas |
| `Managed` | DP manages the application. `False` in [maintenance mode](../../architecture/marketplace/lifecycle.html#maintenance-mode) or if DP can't keep the application in the managed state, for example, because of hook or manifest errors |
| `Ready` | The application is ready to work. During an update, the condition can stay `True` while the previous version keeps working |

Until the initial installation completes, `Installed` is the only condition reported. The other conditions appear once it becomes `True` and are removed again while it is `False`, for example, when a module the application depends on is disabled. While the application is being deleted, all conditions are `False` with the `Deleting` reason.

To quickly view all conditions, use the following command:

```bash
d8 k get applications -n <NAMESPACE> <APPLICATION_NAME> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.status} ({.reason}){"\n"}{end}'
```

Example output:

```console
Installed: True (Installed)
UpdateInstalled: False (Pending)
ConfigurationApplied: True (ConfigurationApplied)
Managed: True (Managed)
Scaled: True (Scaled)
Ready: True (Ready)
```

### Brief state description

The [`status.summary`](../../reference/api/cr.html#application-v1alpha1-status-summary) field provides a brief description of the current application state. Check it first when you diagnose issues:

```yaml
status:
  summary:
    state: Updating
    message: "Update is waiting for dependent modules to converge; previous version is still serving"
    tip: "Wait — the previous version is still working. The update will continue automatically once dependent modules converge."
```

For `status.summary` field descriptions, refer to the [Application](../../reference/api/cr.html#application-v1alpha1-status-summary) documentation.

## Installing multiple instances

The same [package](../../glossary.html#marketplace-package) can be installed multiple times in the same or different namespaces, each with a separate name and settings. For example, two Redis instances can be created: one for caching and one for sessions:

```yaml
# Caching instance.
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-cache
  namespace: team-alpha
spec:
  packageName: redis
  packageRepositoryName: my-registry
  packageVersion: "v7.2.0"
  settings:
    maxmemory: "512mb"
---
# Session storage instance.
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-sessions
  namespace: team-alpha
spec:
  packageName: redis
  packageRepositoryName: my-registry
  packageVersion: "v7.2.0"
  settings:
    maxmemory: "128mb"
```

The objects of each instance are named in the format `d8a-<INSTANCE_NAME>-<SUFFIX>`. For example, `d8a-redis-cache-server` and `d8a-redis-sessions-server`. This prevents any name conflicts.

## Configuring workload resources

The [`spec.resourceRequests`](../../reference/api/cr.html#application-v1alpha1-spec-resourcerequests) parameter overrides the number of replicas, the container resources, and the disk sizes that the package sets for the application workloads. Each entry addresses a workload by its kind (`kind`) and its object name (`name`) in the format `d8a-<INSTANCE_NAME>-<SUFFIX>`. Workloads, containers, and disks that aren't listed keep the package settings. For details on the parameters, refer to the [Application](../../reference/api/cr.html#application) reference.

### Autoscaling

If the package deploys a HorizontalPodAutoscaler or a VerticalPodAutoscaler for a workload, you can fix the values it controls or set a range for them:

- HorizontalPodAutoscaler. If `replicas` is greater than zero, Deckhouse Platform sets this value in the workload and as both HPA bounds. The number of replicas is fixed, and the `ApplicationHorizontalPodAutoscalerPinned` alert is triggered. With `replicas: 0`, the value applies only to the workload, and the HPA bounds remain unchanged.

  To let HPA choose the number of replicas within a range, set `minReplicas` and `maxReplicas`.

- VerticalPodAutoscaler. If a `cpu` or `memory` request is set in `resources.requests` and VPA controls that resource, Deckhouse Platform sets the request in the workload and as both bounds of the VPA policy. The request is fixed, and the `ApplicationVerticalPodAutoscalerPinned` alert is triggered. If a `cpu` or `memory` limit is set, VPA does not change the container's limits; if VPA controls the corresponding resource, the `ApplicationVerticalPodAutoscalerPinned` alert is also triggered.

  To let VPA choose the container's `cpu` or `memory` requests within a range, set `resources.min` and `resources.max`.

Example manifest where HorizontalPodAutoscaler keeps from 2 to 10 replicas of the `d8a-shop-backend` Deployment, VerticalPodAutoscaler chooses the requests of the `backend` container within the range, and the `proxy` container has fixed requests and limits:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: shop
  namespace: my-app
spec:
  packageName: shop
  packageVersion: "v1.4.0"
  packageRepositoryName: my-registry
  resourceRequests:
    - kind: Deployment
      name: d8a-shop-backend
      minReplicas: 2
      maxReplicas: 10
      containers:
        - name: backend
          resources:
            min:
              cpu: 250m
              memory: 256Mi
            max:
              cpu: "2"
              memory: 2Gi
        - name: proxy
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              memory: 256Mi
```

A range always has both bounds and takes the place of the fixed values it bounds. An Application is rejected in the following cases:

- `minReplicas` is set without `maxReplicas`, or the other way round.
- `min` and `max` of a container are not set together for the same resources.
- `min` and `max` are set, but neither contains `cpu` or `memory`.
- `replicas` is set together with `minReplicas` and `maxReplicas`.
- `minReplicas` is greater than `maxReplicas`, or a replicas range is set for a workload other than Deployment, StatefulSet, or ReplicaSet.
- A container with `min` and `max` has `cpu` or `memory` requests or limits (`ephemeral-storage` is allowed, since VerticalPodAutoscaler doesn't manage it).
- `min` of a resource is greater than its `max`.

The error names the workload and the container and explains how to fix the problem. For example:

```console
Application 'my-app/shop' has invalid spec.resourceRequests: Deployment 'd8a-shop-backend', container 'backend': set both min and max: a range needs both bounds, but only min is set
```

To fix a resource of a container with a range at one value, specify the same value in `min` and `max`.

Specifics of ranges:

- Deckhouse Platform doesn't change the number of replicas in the workload itself: within the range, HorizontalPodAutoscaler chooses it.
- Until VerticalPodAutoscaler makes its first recommendation, the pods run with the requests from the package.

If no autoscaler of the application applies a range, for example, because the package doesn't deploy a HorizontalPodAutoscaler for the workload or the `vertical-pod-autoscaler` module is disabled, Deckhouse Platform applies the range as fixed values: the workload runs `minReplicas` replicas, and the container gets `min` as the request and `max` as the limit of the resource. In this case, the `ApplicationHorizontalPodAutoscalerMissing` or `ApplicationVerticalPodAutoscalerMissing` alert is triggered. To fix it, make the autoscaler available or replace the range with fixed values.

If the package doesn't allow changing the number of replicas of a workload, Deckhouse Platform doesn't apply the change: the Application conditions get the `ScalingForbidden` reason, and the message names the workload. To fix it, change or remove the replicas or the range of the workload in `spec.resourceRequests`.

## Updating an application

You can change the application version manually or configure automatic updates. In both cases, transitions between versions are limited by the [version compatibility rules](#version-compatibility-rules) of the package.

### Manual update

To update an application, change [`spec.packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion) to the target version and apply the change:

```bash
d8 k patch applications -n <NAMESPACE> <APPLICATION_NAME> --type=merge -p '{"spec":{"packageVersion":"v7.3.0"}}'
```

While the update is in progress, the `UpdateInstalled` condition is `False` with the `Pending` reason, and then with the `ApplyingManifests` reason while the manifests of the new version are applied. Once the update succeeds, the condition becomes `True`. The previous version keeps working until the update completes.

If the specified version doesn't exist in the repository, the change is rejected, and the current version keeps running. If Deckhouse Platform (DP) fails to download the new version, `UpdateInstalled` becomes `False` with the `DownloadFailed` reason, and the current version keeps running.

If the version compatibility rules don't allow moving from the current version to the specified one in a single step, the change is also rejected, with the `incompatible version` error. In this case, update the application step by step, through intermediate versions.

### Automatic updates

Update settings are set only in the application itself, in the [`spec.update`](../../reference/api/cr.html#application-v1alpha1-spec-update) parameter. There are no common update settings for all applications in the cluster.

The [update mode](../../reference/api/cr.html#application-v1alpha1-spec-update-mode) is set in the `spec.update.mode` parameter:

- `Manual`: Automatic updates are disabled, and the version changes only manually. This mode is used if `spec.update` isn't set.
- `AutoPatch`: Deckhouse Platform automatically installs new patch versions within the current minor version. The application is moved to a new minor or major version manually.
- `Auto`: Deckhouse Platform automatically updates the application to every new version that the version compatibility rules allow.

Example of a manifest in which Redis is automatically updated to new patch versions on Tuesdays and Saturdays from 8:00 to 15:00 UTC:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-cache
  namespace: my-app
spec:
  packageName: redis
  packageVersion: "v7.2.0"
  packageRepositoryName: my-registry
  update:
    mode: AutoPatch
    windows:
      - from: "8:00"
        to: "15:00"
        days:
          - Tue
          - Sat
```

Automatic updates work as follows:

- In every update mode, Deckhouse Platform builds the update path to the newest package version in the cluster that the version compatibility rules allow reaching, and publishes it in [`status.update.plan`](../../reference/api/cr.html#application-v1alpha1-status-update-plan). In the `Auto` and `AutoPatch` modes, the version that the automatic update installs next is published in [`status.update.automaticVersion`](../../reference/api/cr.html#application-v1alpha1-status-update-automaticversion).
- The update goes step by step: the next version is chosen only after the previous one is installed and the application is ready (the `Ready` condition).
- Automatic updates don't change `spec.packageVersion`. In the `Auto` and `AutoPatch` modes, the application runs the newer of `spec.packageVersion` and `status.update.automaticVersion`.

The [update windows](../../reference/api/cr.html#application-v1alpha1-spec-update-windows) set in `spec.update.windows` apply in the `Auto` and `AutoPatch` modes: a new version, including one set in `spec.packageVersion` manually, is installed only during an update window. While the application waits for a window, the `UpdateInstalled` condition is `False` with the `WaitingWindow` reason. If no windows are set, updates are installed at any time. In the `Manual` mode, update windows don't apply. The first installation of an application doesn't wait for a window.

Update notifications can't be configured for an application: the [`update.notification`](/modules/deckhouse/configuration.html#parameters-update-notification) settings of the `deckhouse` module are used. If they set a webhook, in the `Auto` and `AutoPatch` modes Deckhouse Platform sends it a notification about the pending application update (`subject: Application`) and installs the update no earlier than `minimalNotificationTime` after the notification. A patch version update is announced only with `releaseType: All`. If the notification can't be sent, Deckhouse Platform retries, and the update waits. Until the notification is sent and `minimalNotificationTime` has passed, the `UpdateInstalled` condition is `False` with the `WaitingNotification` reason. The sent notification is shown in [`status.update.notified`](../../reference/api/cr.html#application-v1alpha1-status-update-notified).

### Rolling back to an earlier version

To roll back an application, specify an earlier version in `spec.packageVersion`. By default, the version compatibility rules allow rolling back only to patch versions within the current minor version. In the `Auto` and `AutoPatch` modes, the application runs the newer of `spec.packageVersion` and `status.update.automaticVersion`, so switch the application to the `Manual` mode before rolling it back.

{% alert level="warning" %}
Deckhouse Platform doesn't apply any migration logic on rollback. If necessary, verify that the settings are compatible with the target version before applying the change.
{% endalert %}

### Version compatibility rules

Each package version can set version compatibility rules, `versionCompatibilityRules`, in its `package.yaml` file. They define which versions an application can be updated or rolled back to in a single step. The rules apply both to manual changes of `spec.packageVersion` and to automatic updates.

The limits are set separately for updates (`upgrade.step`) and rollbacks (`downgrade.step`):

- `patches`: How many patch versions the application can move within a minor version.
- `minor`: How many minor versions the application can move within a major version.
- `major`: How many major versions the application can move.

The `-1` value removes the limit, `0` forbids the transition, and a positive number sets the largest allowed difference between version numbers. Only the limit of the highest changed part of the version is checked: `major` when the major version changes, and `minor` when the minor version changes. The rules of the version being left apply, except for the `upgrade.from` constraint described below. The difference is counted by version numbers, not by the number of published versions: with `minor: 1`, the application can't move from `v1.2.3` to `v1.4.0` directly, even if there are no `v1.3.*` versions.

If a limit isn't set, its default value is used:

| Limit | `upgrade.step` | `downgrade.step` |
|---|---|---|
| `patches` | `-1`: Any patch version within the minor version | `-1`: Any patch version within the minor version |
| `minor` | `1`: The next minor version | `0`: Rolling back the minor version is forbidden |
| `major` | `0`: Moving to another major version is forbidden | `0`: Rolling back the major version is forbidden |

The effective rules of a version, with unset limits replaced by their default values, are published in [`status.packageMetadata.versionCompatibilityRules`](../../reference/api/cr.html#applicationpackageversion-v1alpha1-status-packagemetadata-versioncompatibilityrules) of the ApplicationPackageVersion object:

```bash
d8 k get apv my-registry-redis-v7.2.0 -o jsonpath='{.status.packageMetadata.versionCompatibilityRules}'
```

Example of the rules in `package.yaml`:

```yaml
versionCompatibilityRules:
  upgrade:
    step:
      # Update up to three minor versions ahead in a single step.
      minor: 3
      # Update to the next major version.
      major: 1
  downgrade:
    step:
      # Roll back up to five patch versions within a minor version.
      patches: 5
      # Roll back to the previous minor version.
      minor: 1
```

The other limits in the example (`upgrade.step.patches` and `downgrade.step.major`) take their default values.

#### upgrade.from constraint

The `step` limits are set in the version being left and don't change after it's published. Therefore, earlier versions can't allow moving to a version released later, such as a new major version. A new version can specify which versions it can be updated from: its `package.yaml` sets the `upgrade.from` constraint in the SemVer format:

```yaml
# package.yaml of version v2.0.0.
versionCompatibilityRules:
  upgrade:
    from: ">= 1.5.0"
```

If `upgrade.from` is set, it takes priority over the `upgrade.step` limits of the version being updated from: the application can be updated to this version only from versions that satisfy the constraint. The constraint allows transitions that the limits forbid and forbids transitions that they allow. In the example, the application is updated to `v2.0.0` from any version starting from `v1.5.0`, although moving to another major version is forbidden by default. From versions below `v1.5.0`, the application is updated through intermediate versions.

An invalid constraint forbids updating to this version. `upgrade.from` doesn't affect rollbacks: a rollback follows the `downgrade.step` limits of the version being rolled back from.

## Deleting an application

To delete an application, delete the Application object. For example:

```bash
d8 k delete applications -n <NAMESPACE> <APPLICATION_NAME>
```

When an Application is deleted, DP deletes the Kubernetes objects of the application, except for the objects that the package templates protect with the `helm.sh/resource-policy: keep` annotation. The objects that the application creates at runtime and the objects that stay after the Helm release is uninstalled (for example, Helm hooks), are deleted if the package declares them as [orphan resources](../../architecture/marketplace/lifecycle.html#orphan-resources). The Application stays in the cluster until DP completes the deletion.

## FAQ

### Can an application be updated automatically?

Yes. Set the `Auto` or `AutoPatch` mode in the [`spec.update.mode`](../../reference/api/cr.html#application-v1alpha1-spec-update-mode) parameter. For details, see [Automatic updates](#automatic-updates).

### Can an Application depend on another Application?

No. An Application can declare dependencies only on modules (via `requirements.modules` in `package.yaml`). This is an architectural constraint that ensures instance isolation.

### Can I install the same application in different namespaces?

Yes. Create Application objects with the same `packageName` and `packageVersion` in different namespaces. Each of them is a fully independent instance.
