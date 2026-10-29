---
title: Installing and managing applications
permalink: en/user/marketplace/applications.html
description: "Install, update, and delete applications in Deckhouse Platform Marketplace. Browse available package versions, create Application, check status conditions, and manage multiple instances."
lang: en
search: Application, install application, application conditions, installing application, updating application, deleting application
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

## Updating an application

Applications are updated manually. To update an application, change [`spec.packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion) to the target version and apply the change:

```bash
d8 k patch applications -n <NAMESPACE> <APPLICATION_NAME> --type=merge -p '{"spec":{"packageVersion":"v7.3.0"}}'
```

While the update is in progress, the `UpdateInstalled` condition is `False` with the `Pending` reason, and then with the `ApplyingManifests` reason while the manifests of the new version are applied. Once the update succeeds, the condition becomes `True`. The previous version keeps working until the update completes.

If the specified version doesn't exist in the repository, the change is rejected, and the current version keeps running. If Deckhouse Platform (DP) fails to download the new version, `UpdateInstalled` becomes `False` with the `DownloadFailed` reason, and the current version keeps running.

{% alert level="warning" %}
You can specify an older version to perform a downgrade, but DP doesn't apply any migration logic on rollback. If necessary, verify that the settings are compatible with the target version before applying the change.
{% endalert %}

## Deleting an application

To delete an application, delete the Application object. For example:

```bash
d8 k delete applications -n <NAMESPACE> <APPLICATION_NAME>
```

When an Application is deleted, DP deletes the Kubernetes objects of the application, except for the objects that the package templates protect with the `helm.sh/resource-policy: keep` annotation. The objects that the application creates at runtime and the objects that stay after the Helm release is uninstalled (for example, Helm hooks), are deleted if the package declares them as [orphan resources](../../architecture/marketplace/lifecycle.html#orphan-resources). The Application stays in the cluster until DP completes the deletion.

## FAQ

### Can an application be updated automatically?

No. In the current implementation, updates require a manual change to [`spec.packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion). Automatic application updates via [release channels](../../glossary.html#release-channel) are planned for future versions.

### Can an Application depend on another Application?

No. An Application can declare dependencies only on modules (via `requirements.modules` in `package.yaml`). This is an architectural constraint that ensures instance isolation.

### Can I install the same application in different namespaces?

Yes. Create Application objects with the same `packageName` and `packageVersion` in different namespaces. Each of them is a fully independent instance.
