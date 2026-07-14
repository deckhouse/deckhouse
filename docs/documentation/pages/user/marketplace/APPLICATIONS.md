---
title: Installing and managing applications
permalink: en/user/marketplace/applications.html
description: "Install, update, and delete applications in Deckhouse Kubernetes Platform Marketplace. Browse available package versions, create Application, check status conditions, and manage multiple instances."
lang: en
search: Application, install application, application conditions, installing application, application conditions, updating application
---

## Browsing available package versions

To list all available package versions, use the following command (the short name `apv` can be used):

```bash
d8 k get apv
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                        PACKAGE   REPOSITORY    TRANSITIONTIME   METADATALOADED   USEDBY
my-registry-redis-v7.2.0    redis     my-registry   2d               True             1
my-registry-redis-v7.3.0    redis     my-registry   5h               True
my-registry-pg-v15.0.0      postgres  my-registry   2d               True             2
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

To filter by package name, use the following command (the example filters versions of the `redis` package):

```bash
d8 k get apv -l package=redis
```

To see which registries provide a specific package, use the following command (the short name `ap` can be used):

```bash
d8 k get ap redis \
  -o jsonpath='{.status.availableRepositories}'
```

{% alert level="info" %}
Only versions with `MetadataLoaded=True` can be installed. This means the package's OpenAPI schema, description, and requirements were successfully loaded from the registry. A package with `MetadataLoaded=False` cannot be installed until the metadata is retrieved.
{% endalert %}

## Installing an application

To install an application, create an [Application](../../reference/api/cr.html#application) object in the desired namespace.

Example manifest for installing Redis from the `redis` package version `v7.2.0` with `maxmemory` settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Application
metadata:
  name: redis-cache
  namespace: my-app
spec:
  packageName: redis
  packageVersion: "v7.2.0"
  # Can be omitted if only one repository provides this package
  packageRepositoryName: my-registry
  settings:
    replicas: 3
    maxmemory: "256mb"
```

{% alert level="info" %}
`spec.settings` is validated against the OpenAPI schema defined in the package. If the schema rejects your settings, the Application will not be created. Check the package's [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) for schema details.
{% endalert %}

### Naming constraints

The Application name (`metadata.name`) must be **at most 24 characters**. This is required because all pods are prefixed with the instance name: a 24-char instance name + 24-char resource name + 15-char Deployment suffix fits within the Kubernetes pod name limit of 63 characters.

## Checking application status

To get a brief status of an application, use the following command (the short name `app` can be used):

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME>
```

Example output:

```console
NAME          PACKAGE   VERSION   INSTALLED   READY   MESSAGE
redis-cache   redis     v7.2.0    True        True
```

To get the full status including conditions, use the following command:

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> -o yaml
```

### Conditions

The application state is described in detail through a set of conditions:

| Condition | Meaning |
|---|---|
| `Installed` | Package downloaded, manifests and hooks applied for the initial installation |
| `UpdateInstalled` | New version downloaded, manifests and hooks applied for the update |
| `ConfigurationApplied` | User settings successfully applied |
| `Scaled` | All pod replicas are in Ready state |
| `Managed` | Application is correctly managed by DKP |
| `Ready` | Application is fully operational |

To quickly view all conditions, use the following command:

```bash
d8 k get app -n <NAMESPACE> <APPLICATION_NAME> \
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

### Summary

The `status.summary` field provides a brief description of the current application state — useful to check first when diagnosing issues:

```yaml
status:
  summary:
    state: Updating
    message: "Update is waiting for dependent modules to converge; previous version is still serving"
    tip: "Waiting until DKP processes all dependent modules to start the update."
```

- **`state`** — current high-level state of the application.
- **`message`** — explains why the application is in this state.
- **`tip`** — what to do to resolve the issue or what DKP is waiting for.

## Multiple instances

The same package can be installed multiple times in the same or different namespaces — each with a separate name and settings. For example, two Redis instances can be created — one for caching, one for sessions:

```yaml
# Caching instance
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
# Session storage instance
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

All Kubernetes objects created by an Application are prefixed with the instance name — for example, `redis-cache-deployment` and `redis-sessions-deployment` — so names never conflict.

## Updating an application

Updates are manual: change `spec.packageVersion` to the desired version and apply the change:

```bash
d8 k patch app -n <NAMESPACE> <APPLICATION_NAME> --type=merge -p '{"spec":{"packageVersion":"v7.3.0"}}'
```

While the update is in progress, the `UpdateInstalled` condition will be `False` with `reason: Pending`. Once the update succeeds, it becomes `True`. The previous version continues serving until the update completes.

If the specified version does not exist in the repository, `UpdateInstalled` becomes `False` with `reason: UpdateFailed`, and the current version keeps running.

{% alert level="warning" %}
Specifying an older version (downgrade) is allowed, but DKP does not apply any migration logic on rollback. Verify settings compatibility with the target version before applying the change, if necessary.
{% endalert %}

## Deleting an application

To delete an application, delete the Application object. For example:

```bash
d8 k delete app -n <NAMESPACE> <APPLICATION_NAME>
```

When an Application is deleted, all Kubernetes objects it created (unless protected by `helm.sh/resource-policy: keep` or `werf.io/ownership: anyone` annotations in the package templates) will also be deleted.

## FAQ

### Can updates happen automatically?

No. In the current implementation, updates require a manual change to `spec.packageVersion`. Automatic updates via release channels are planned for future versions.

### Can an Application depend on another Application?

No. An Application can only declare dependencies on Modules (via `requirements.modules` in `package.yaml`). This is an architectural constraint that ensures instance isolation.

### Can I install the same application in different namespaces?

Yes. Create Application objects with the same `packageName` and `packageVersion` in different namespaces — each will be a fully independent instance.
