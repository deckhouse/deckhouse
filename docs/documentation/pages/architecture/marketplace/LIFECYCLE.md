---
title: Lifecycle and debugging
permalink: en/architecture/marketplace/lifecycle.html
description: "How Deckhouse Platform installs, updates, reconciles, and deletes an Application: admission checks, requirements, the order of hooks and Helm operations, deletion and orphan resources, resource restoration, deployment tracking, maintenance mode, and debugging tools."
---

This page describes how Deckhouse Platform (DP) runs an application instance and which tools help to debug a package.

## Admission checks

When an [Application](../../reference/api/cr.html#application) is created or changed, the DP validating webhook checks that:

- the Application name is at most 24 characters long;
- the ApplicationPackage exists, and the ApplicationPackageVersion for the specified repository, package, and version exists and has its metadata loaded;
- the settings match the settings schema of the package version (see [Application settings](settings.html#when-settings-are-validated));
- the cluster meets the [package requirements](application-development.html#requirements): the Kubernetes and DP versions and the module dependencies. The rejection message names the unmet requirement, the required value, and the value in the cluster.

## Installation pipeline

DP processes each application instance in a separate queue, step by step:

1. **Download.** DP downloads the package bundle of the specified version from the repository.
2. **Load.** DP reads `package.yaml`, the schemas, and `images_digests.json`, and discovers the hooks.
3. **Scheduling.** DP waits until the package requirements are met: the cluster bootstrap is complete, the Kubernetes and DP versions match the constraints, and the modules from `requirements.modules` are enabled. While DP waits for the modules the application depends on, the Application summary reports it.
4. **Configuration.** DP validates and applies the settings.
5. **Hook initialization.** DP enables the `schedule` bindings and the Kubernetes monitors of the hooks and runs the `Synchronization` of the Kubernetes bindings. The first time after the package is loaded, DP also runs `onStartup` hooks.
6. **Apply.** DP runs `beforeHelm` hooks, renders the templates and applies them with Nelm, and then runs `afterHelm` hooks. If `afterHelm` hooks change values, DP applies the templates again.

Steps 4–6 form a reconciliation run. In the apply step, DP upgrades the Helm release only if it is needed: on the first installation, if the last release revision is not deployed, if the rendered manifests have changed, or if some resources are missing in the cluster.

The order of hook bindings and the handling of hook errors are described in [Hooks](hooks.html#bindings-and-execution-order).

## Events and reactions

| Event | Reaction of DP |
|---|---|
| The `spec.settings` or `spec.maintenance` field of the Application changes | A new reconciliation run |
| A `schedule` or Kubernetes hook changes values | A new reconciliation run |
| The `spec.packageVersion` field changes | DP stops the hooks of the current version while keeping its resources, and then goes through all steps with the new version. The templates of the new version are applied as an upgrade of the same Helm release |
| DP restarts | DP loads the package again and goes through all steps, including `onStartup` hooks |
| A resource of the application is deleted from the cluster | DP restores it (see [Resource restoration](#resource-restoration)) |
| The package requirements stop being met, for example, a module from `requirements.modules.mandatory` is disabled | DP uninstalls the application but keeps the Application (see [Suspension](#suspension)) |
| The Application is deleted | DP uninstalls the application, deletes its [orphan resources](#orphan-resources), and then removes the Application (see [Deletion](#deletion)) |

## Suspension

DP checks the package requirements not only on admission but all the time. If the requirements of an installed application stop being met, DP:

1. Runs `beforeDeleteHelm` hooks.
2. Uninstalls the Helm release, which deletes the resources of the application.
3. Runs `afterDeleteHelm` hooks.

The Application stays in the cluster in the `Suspended` state, and the [orphan resources](#orphan-resources) of the application are not deleted. When the requirements are met again, DP installs the application again with the same settings.

{% alert level="warning" %}
The resources are deleted together with the Helm release, including PersistentVolumeClaims. To keep the data, add the `helm.sh/resource-policy: keep` annotation to PersistentVolumeClaims (see [Nelm annotations](nelm-annotations.html#2-preserve-a-resource-across-uninstall-or-chart-removal)).
{% endalert %}

## Deletion

When an Application is deleted, DP:

1. Runs `beforeDeleteHelm` hooks.
2. Uninstalls the Helm release and waits until its resources are deleted.
3. Runs `afterDeleteHelm` hooks and stops the hooks: disables the `schedule` bindings and the Kubernetes monitors.
4. Deletes the [orphan resources](#orphan-resources) of the application.
5. Removes the Application.

If a step fails, DP retries it with an increasing delay, from 15 seconds up to 2 hours. The Application stays in the cluster until all steps succeed.

### Orphan resources

Uninstalling the Helm release deletes only the objects of the release. The following objects stay in the namespace:

- objects that the workloads or hooks of the application create at runtime;
- Helm hooks, that is, objects of the templates with the `helm.sh/hook` annotation, if they are still in the cluster: Nelm doesn't delete hooks when it uninstalls a release (see [Nelm annotations](nelm-annotations.html#lifecycle-annotations)).

To make DP delete such objects together with the Application, list their kinds in the `orphanResources` field of the values, as the default value of the field in `openapi/values.yaml`. For example:

```yaml
x-extend:
  schema: settings.yaml
type: object
properties:
  orphanResources:
    type: array
    default:
      - kind: Job
        version: batch/v1
      - kind: ConfigMap
        version: v1
    items:
      type: object
      required: [kind, version]
      properties:
        kind:
          type: string
        version:
          type: string
```

Each item of the list contains the following fields:

- `kind` — kind of the objects, for example, `Job`.
- `version` — API version of the kind: `<GROUP>/<VERSION>`, for example, `batch/v1`, or only `<VERSION>`, for example, `v1`. A version without a group matches the kind in every API group that serves it at this version, so `kind: Job` with `version: v1` matches the `batch/v1` Jobs too. To match the kind in one group only, specify the group.

Declare the field in `openapi/values.yaml` rather than in `openapi/settings.yaml`, so that users can't change the list in `spec.settings`. DP reads the list from the values of the current package version when the Application is deleted. If earlier versions of the package created objects of other kinds, keep these kinds in the list.

DP deletes the objects of the listed kinds that are located in the namespace of the Application and carry the `packages.deckhouse.io/instance: <INSTANCE_NAME>` label. The object names don't matter. DP adds the label to every object rendered from the templates, including Helm hooks (see [Templates](templates.html#labels-and-object-protection)). Objects created by workloads and hooks don't get the label automatically, so set it yourself. For example, a hook sets the label to the instance name:

```go
input.PatchCollector.CreateOrUpdate(&corev1.ConfigMap{
    TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
    ObjectMeta: metav1.ObjectMeta{
        Name:   "state",
        Labels: map[string]string{"packages.deckhouse.io/instance": input.Instance.Name()},
    },
    Data: map[string]string{"instance": input.Instance.Name()},
})
```

A workload can get the instance name from the templates (`.Application.Instance.Name`), for example, in an environment variable.

DP deletes the objects after the Helm release is uninstalled and the hooks are stopped, so that the application can't create them again. Dependent objects are deleted in the background: for example, the Kubernetes garbage collector deletes the Pods of a deleted Job, so their kind doesn't need to be listed. DP doesn't wait until the objects are gone: an object with finalizers stays in the cluster until its finalizers are removed, even after the Application is deleted.

DP doesn't delete:

- Objects that must outlive the release: objects with the `helm.sh/resource-policy: keep` or `werf.io/ownership: anyone` annotation, and objects whose `werf.io/resource-policy` annotation contains `keep` or `skip-delete`. As in Nelm, `helm.sh/resource-policy` is ignored if `werf.io/resource-policy` is set. For example, the Job from [Resource rendered only on first install](nelm-annotations.html#6-resource-rendered-only-on-first-install) stays after the Application is deleted. Helm hooks without these annotations are deleted.
- Objects that are already being deleted.
- Objects of cluster-wide kinds and of kinds that the cluster doesn't serve, for example, custom resources whose CustomResourceDefinition is not installed. DP skips such kinds and writes a warning to its log.

DP deletes orphan resources only when the Application is deleted. When the application is [suspended](#suspension) or its package version changes, the objects stay, because the application can use them again. If the package of the application isn't loaded when the Application is deleted, for example, because DP failed to download the package bundle, DP uninstalls the Helm release but doesn't delete orphan resources.

If DP fails to list or delete the objects, for example, when the API server is unavailable, DP deletes the rest of the objects and retries the step. If the `orphanResources` value is not a list of kinds, DP writes an error to its log and removes the Application without deleting orphan resources.

DP writes every deleted object and every skipped kind to its log: search the [log](#logs) for `orphan`. While DP retries the step, the queue of the application contains the `Purge` task with the error (see [State in DP](#state-in-dp)).

## Resource restoration

DP remembers the resources rendered from the templates and checks every 4–5 minutes that all of them exist in the cluster. If a resource is missing, DP starts a reconciliation run, and the resource is created again.

DP checks only whether the resources exist, not their contents. The check is paused while hooks run and is disabled in [maintenance mode](#maintenance-mode).

## Deployment tracking

When DP applies the templates, Nelm waits until the resources become ready. The waiting is controlled by the [tracking annotations](nelm-annotations.html#tracking-annotations). Pod logs are not collected, so the [log annotations](nelm-annotations.html#log-annotations) have no effect.

The progress of the current apply is published in the `status.tracking` field of the Application: a list of Nelm operations with their states and dependencies.

One apply is limited to 30 minutes. When the limit is exceeded, DP cancels the apply, and the condition message names up to five resources DP was waiting for, for example:

```text
install nelm release 'my-namespace.myapp': apply timed out after 30m0s, waiting for Job/d8a-myapp-migrate
```

Whether the application is running after the apply is determined from its Deployments and StatefulSets (see [Workload health](templates.html#workload-health)).

## Maintenance mode

To debug or tweak an installed application manually, switch it to maintenance mode:

```yaml
spec:
  maintenance: NoResourceReconciliation
```

In maintenance mode:

- DP stops applying the templates: changes of settings and values are not reflected in the resources. Hooks keep running.
- Deleted resources are not restored.
- DP labels the resources rendered from the templates with `maintenance.deckhouse.io/no-resource-reconciliation`, and the admission policies stop protecting them, so the resources can be changed manually. To add the label, DP applies the templates once when the mode is enabled.
- The `ApplicationIsInMaintenanceMode` alert fires.

To return the application to normal operation, remove the `spec.maintenance` field. DP applies the templates, which restores the managed state of the resources and removes the label.

## Debugging

### Application status

The following fields of the Application status are useful when you develop a package:

| Field | Description |
|---|---|
| `status.summary` | High-level state of the application, the reason, and a hint on what to do |
| `status.conditions` | Detailed conditions (see [Installing and managing applications](../../user/marketplace/applications.html#conditions)) |
| `status.currentVersion.version` | Installed package version |
| `status.lastAppliedConfiguration` | Effective settings of the last successful apply |
| `status.urls` | [Application endpoints](templates.html#application-endpoints) |
| `status.tracking` | Nelm progress report of the current or the last apply |

Example:

```bash
d8 k -n <NAMESPACE> get applications <INSTANCE_NAME> -o jsonpath='{.status.summary}'
```

### State in DP

The `deckhouse-controller packages` commands show the state of the application in DP. The package name in these commands is `<NAMESPACE>.<INSTANCE_NAME>`. Run the commands in the `deckhouse` container of the DP leader pod.

{% alert level="warning" %}
The output contains values and container registry credentials. Only cluster administrators should have access to it.
{% endalert %}

- Render the templates with the current values:

  ```bash
  d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages render <NAMESPACE>.<INSTANCE_NAME>
  ```

- Show the loaded package: path, definition, image digests, repository, values, hooks, and internal conditions:

  ```bash
  d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages dump --name <NAMESPACE>.<INSTANCE_NAME>
  ```

- Show the snapshots of the hook Kubernetes bindings:

  ```bash
  d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages snapshots <NAMESPACE>.<INSTANCE_NAME>
  ```

- Show the task queues of the application:

  ```bash
  d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages queue dump --name <NAMESPACE>.<INSTANCE_NAME>
  ```

- Show the scheduler state of the application, for example, which requirement is not met:

  ```bash
  d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages scheduler dump --name <NAMESPACE>.<INSTANCE_NAME>
  ```

### Logs

DP writes the application processing events, including the output of hooks, to its log:

```bash
d8 k -n d8-system logs svc/deckhouse-leader -c deckhouse | grep '<NAMESPACE>.<INSTANCE_NAME>'
```
