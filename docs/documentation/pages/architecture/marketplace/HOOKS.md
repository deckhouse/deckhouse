---
title: Hooks
permalink: en/architecture/marketplace/hooks.html
description: "Go hooks for Deckhouse Platform Marketplace Applications: module-sdk hooks binary, bindings and execution order, ApplicationHookInput, Kubernetes bindings, object operations, values, and the settings validation hook."
---

Application hooks are written in Go using [module-sdk](https://github.com/deckhouse/module-sdk). Deckhouse Platform (DP) runs them at certain stages of the application lifecycle, on changes of Kubernetes objects, and on schedule. Hooks prepare values for templates, create and change objects in the application namespace, and validate settings.

## Hooks binary

All hooks of a package are compiled into a single Go binary. In the package skeleton, the sources are located in `hooks/batch/`, and `d8 package build` builds the binary according to `hooks/hooks.yaml` and puts it into the package bundle.

Each hook is registered with `registry.RegisterFunc`, and the `main` function calls `app.Run`:

```go
// hooks/batch/main.go
package main

import (
    "github.com/deckhouse/module-sdk/pkg/app"

    "hooks/settings"
    _ "hooks/triggers"
)

func main() {
    app.Run(app.WithSettingsCheck(settings.Check))
}
```

Requirements for the binary:

- The hook source files must be located under a directory named `hooks`: module-sdk builds the hook name from the file path.
- The binary must register at least one hook. DP ignores a binary without hooks, even if it contains a [settings validation hook](#settings-validation-hook).
- Build a static binary for Linux (`CGO_ENABLED=0`), as the skeleton does.

DP finds hooks by running each executable file without an extension from the `hooks/` directory of the bundle with the `hook list` argument. A binary that fails at this stage, for example, because of an invalid hook configuration, is skipped without an error in the Application status. To make sure that the hooks are registered, check the output of the following commands in CI:

```bash
cd hooks/batch
CGO_ENABLED=0 go build -o /tmp/hooks .
/tmp/hooks hook list
/tmp/hooks hook config
```

## Hook example

```go
package state

import (
    "context"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    "github.com/deckhouse/module-sdk/pkg"
    objectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
    "github.com/deckhouse/module-sdk/pkg/registry"
)

var _ = registry.RegisterFunc(&pkg.ApplicationHookConfig{
    OnBeforeHelm: &pkg.OrderedConfig{Order: 10},
    Kubernetes: []pkg.ApplicationKubernetesConfig{
        {
            Name:       "credentials",
            APIVersion: "v1",
            Kind:       "Secret",
            LabelSelector: &metav1.LabelSelector{
                MatchLabels: map[string]string{"example.com/credentials": "true"},
            },
            JqFilter: `{"name": .metadata.name}`,
        },
    },
}, handle)

type secret struct {
    Name string `json:"name"`
}

func handle(_ context.Context, input *pkg.ApplicationHookInput) error {
    secrets, err := objectpatch.UnmarshalToStruct[secret](input.Snapshots, "credentials")
    if err != nil {
        return err
    }

    // The path must be declared in openapi/values.yaml.
    input.Values.Set("internal.credentialsCount", len(secrets))

    // The ConfigMap is created as d8a-<INSTANCE_NAME>-state in the application namespace.
    input.PatchCollector.CreateOrUpdate(&corev1.ConfigMap{
        TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
        ObjectMeta: metav1.ObjectMeta{Name: "state"},
        Data:       map[string]string{"instance": input.Instance.Name()},
    })

    return nil
}
```

## Bindings and execution order

The `pkg.ApplicationHookConfig` structure defines when the hook runs:

| Field | When the hook runs |
|---|---|
| `OnStartup` | Once after the package is loaded: on installation, after the package version changes, and after DP restarts. The hook doesn't get snapshots |
| `OnBeforeHelm` | In every reconciliation run, before the templates are applied |
| `OnAfterHelm` | In every reconciliation run, after the templates are applied. If the hook changes values, DP applies the templates again |
| `OnBeforeDeleteHelm` | Before the Helm release is uninstalled: when the Application is deleted or when the package requirements stop being met. The hook doesn't run when the package version changes |
| `OnAfterDeleteHelm` | After the Helm release is uninstalled, in the same cases as `OnBeforeDeleteHelm` |
| `Schedule` | On schedule in the crontab format |
| `Kubernetes` | When the watched objects in the application namespace change, and during the synchronization of the binding (see [Kubernetes bindings](#kubernetes-bindings)) |

Other fields:

- `Queue` — name of the queue for the `Schedule` and `Kubernetes` bindings of the hook (default: `main`). These bindings run in a queue separate from the lifecycle stages, so they can run at the same time as `OnBeforeHelm` or `OnAfterHelm` hooks.
- `AllowFailure` and `Settings` are not supported for applications.

Hooks with the same binding run in the ascending order of `Order` (the `pkg.OrderedConfig` field). The sequence of lifecycle stages is described in [Lifecycle and debugging](lifecycle.html#installation-pipeline).

A hook can't combine `OnStartup` with `Kubernetes` bindings: module-sdk panics when such a hook is registered, and DP skips the whole binary.

### Error handling

- If a hook fails, the stage that ran it fails and is retried with an increasing delay, from 15 seconds up to 2 hours. The next tasks of the application wait. The error is reported in the Application conditions and summary.
- If an `OnBeforeDeleteHelm` hook fails, DP doesn't uninstall the Helm release and retries the hook. The Application is not deleted until the hook succeeds.
- If an object operation of the hook fails, the hook run fails, and the values changes made by the run are discarded. The operations that have already been executed are not rolled back.

{% alert level="warning" %}
module-sdk recovers panics in hook handlers, and the run is considered successful but has no effect: neither values changes nor object operations are applied. Return errors instead of panicking.
{% endalert %}

## ApplicationHookInput

The hook handler has the `func(ctx context.Context, input *pkg.ApplicationHookInput) error` signature. `pkg.ApplicationHookInput` contains the following fields:

| Field | Description |
|---|---|
| `Instance` | Instance parameters: `Name()` and `Namespace()` |
| `Snapshots` | Snapshots of the Kubernetes bindings of the hook |
| `Values` | Application values: reading (`Get`, `GetOk`, `Exists`, `ArrayCount`) and changing (`Set`, `Remove`) |
| `Settings` | Effective settings of the application, read-only |
| `PatchCollector` | Operations with objects in the application namespace (see [Object operations](#object-operations)) |
| `DC` | Dependencies: an HTTP client (`GetHTTPClient`), a container registry client (`GetRegistryClient`), and a clock (`GetClock`) |
| `Logger` | Logger. The output is written to the DP log |
| `MetricsCollector` | Not supported for applications: DP ignores the metrics |

Application hooks don't get a Kubernetes client. To read objects, use Kubernetes bindings, and to change them, use `PatchCollector`.

The instance name and namespace are the only parameters of the instance available to hooks. The package name, version, and images are available only to [templates](templates.html#application).

## Kubernetes bindings

The `pkg.ApplicationKubernetesConfig` structure describes the objects the hook watches:

| Field | Description |
|---|---|
| `Name` | Binding name, the key in `Snapshots` |
| `APIVersion`, `Kind` | API version and kind of the objects. Only namespaced kinds are supported |
| `NameSelector`, `LabelSelector`, `FieldSelector` | Selection of objects |
| `JqFilter` | jq expression applied to each object. The snapshot contains the results of the expression. Without the expression, the snapshot contains full objects |
| `ExecuteHookOnEvents` | `false` — update the snapshot without running the hook when the objects change. Default: `true` |
| `ExecuteHookOnSynchronization` | `false` — don't run the hook during the synchronization. Default: `true` |
| `WaitForSynchronization` | `false` — don't wait for the synchronization of the binding before the next stages. It takes effect only if the `Queue` field of the hook is set. Default: `true` |
| `AllowFailure` | Ignore errors of the hook during the synchronization |

Behavior:

- DP always restricts the bindings to the application namespace, so the structure has no namespace selector.
- On every run, the hook gets the snapshots of all its bindings: the current state of the objects, not the event that triggered the run.
- `OnBeforeHelm`, `OnAfterHelm`, `OnBeforeDeleteHelm`, and `OnAfterDeleteHelm` hooks get the snapshots too.
- DP synchronizes the bindings at the beginning of every reconciliation run: hooks with `ExecuteHookOnSynchronization` enabled run with the full list of objects.
- `NameSelector` matches the actual object names, including the `d8a-<INSTANCE_NAME>-` prefix.

The hook configuration is the same for all instances of the package, so selectors can't refer to the instance name. If several instances of the package can be installed into the same namespace, add the instance label to the snapshot and compare it with `input.Instance.Name()`. Objects rendered from the templates carry the `packages.deckhouse.io/instance` label:

```go
JqFilter: `{"name": .metadata.name, "instance": .metadata.labels["packages.deckhouse.io/instance"]}`,
```

## Object operations

`input.PatchCollector` collects the operations with objects. DP executes them after the hook completes:

| Method | Description |
|---|---|
| `Create` | Create an object. The operation fails if the object exists |
| `CreateIfNotExists` | Create an object if it doesn't exist |
| `CreateOrUpdate` | Create an object or replace the existing one. The fields that the hook doesn't set are removed from the object |
| `Delete`, `DeleteInBackground`, `DeleteNonCascading` | Delete an object with foreground, background, or orphan propagation of the deletion. A missing object is ignored |
| `PatchWithJSON`, `PatchWithMerge`, `PatchWithJQ` | Patch an object with a JSON Patch (RFC 6902), a JSON Merge Patch (RFC 7396), or a jq expression. To patch a subresource, pass the `objectpatch.WithSubresource("status")` option |

Behavior:

- module-sdk sets the application namespace in every operation, regardless of the namespace specified in the object.
- Typed objects must have `TypeMeta` with `apiVersion` and `kind` set. Otherwise, the operation is not executed.
- Patching a missing object fails the hook. Check that the object exists, for example, in a snapshot, before patching it.
- Objects created by hooks are not part of the Helm release: DP doesn't delete them when the application is deleted, and doesn't add [platform labels](templates.html#labels-and-object-protection) to them. Delete such objects in `OnBeforeDeleteHelm` or `OnAfterDeleteHelm` hooks, or set an owner reference to an object of the release.

### Object names

DP adds the `d8a-<INSTANCE_NAME>-` prefix to the object name in every operation, unless the name already starts with it. For example, for the `myapp` instance:

- `Create` of the `state` ConfigMap creates `d8a-myapp-state`;
- `PatchWithMerge` of the `server` Deployment changes `d8a-myapp-server`, that is, the Deployment named `d8a-<INSTANCE_NAME>-server` in the templates.

As a result, a hook can change only the objects of its own instance. Objects with the `d8a-` prefix are protected from changes by users (see [Templates](templates.html#labels-and-object-protection)).

## Values

`input.Values` contains the application values: the same document the templates get as `.Values`. The settings are located at the root of the document, without the package name key.

- `Set(path, value)` and `Remove(path)` accept a path with dot-separated keys, for example, `internal.credentialsCount`. The parent object of the path must exist, so declare intermediate objects in `openapi/values.yaml` with `default: {}`.
- After the hook completes, DP validates the values against `openapi/values.yaml`. The schema must include the settings schema with `x-extend` (see [Application settings](settings.html#schema-files)): the settings are located at the root of the values, and undeclared fields fail the validation.
- The values changes are kept in memory. After DP restarts or the package version changes, they are lost until the hooks run again.

What happens after the values change depends on the binding:

| Binding | Result |
|---|---|
| `Kubernetes`, `Schedule` | DP starts a new reconciliation run |
| `OnAfterHelm` | DP applies the templates again |
| `OnStartup`, `OnBeforeHelm`, synchronization | The values are used when the templates are applied in the same run |

`input.Settings` is read-only: hooks can't change the application settings.

## Settings validation hook

Applications can include a settings validation hook for checks that the OpenAPI schema can't express, for example, business logic constraints across multiple settings fields.

The hook implements a `Check` function with the signature:

```go
func Check(_ context.Context, input settingscheck.Input) settingscheck.Result
```

`settingscheck.Input` provides access to the settings (`input.Settings`) with the default values from `openapi/settings.yaml` applied, as well as to a logger (`input.Logger`).

`settingscheck.Result` is one of:

- `settingscheck.Allow(warnings...)` — settings are valid; optionally attach warning messages.
- `settingscheck.Reject(reason)` — settings are invalid.

**Example:**

```go
func Check(_ context.Context, input settingscheck.Input) settingscheck.Result {
    replicas := input.Settings.Get("replicas").Int()
    if replicas == 0 {
        return settingscheck.Reject("replicas cannot be 0")
    }

    var warnings []string
    if replicas == 2 {
        warnings = append(warnings, "an even number of replicas may cause split-brain in some configurations")
    }

    if replicas > 3 {
        return settingscheck.Reject("replicas cannot be greater than 3")
    }

    return settingscheck.Allow(warnings...)
}
```

To register the check, pass it to `app.Run` in the hooks binary: `app.Run(app.WithSettingsCheck(Check))`. A package can contain only one settings validation hook.

DP calls the check in the following cases:

- When the settings of an installed application are changed, and `spec.packageVersion` matches the installed version, the check runs in the validating webhook. A rejection denies the request with the specified reason. Warnings are returned to the client, for example, `d8 k` prints them.
- When DP applies changed settings. A rejection leaves the new settings unapplied, and the error is reported in the Application conditions. Warnings are ignored.

The check doesn't run in the validating webhook when an Application is created or `spec.packageVersion` changes: in these cases, the request is checked against the settings schema only. Describe critical constraints in the schema, for example, with [CEL rules](settings.html#cel-rules-x-deckhouse-validations), to reject invalid settings before installation.

## Restrictions

- Kubernetes bindings and object operations are limited to the application namespace, and object operations are also limited to the objects with the `d8a-<INSTANCE_NAME>-` name prefix.
- Hooks can't create or change cluster-wide objects.
- Readiness probes (`app.WithReadiness`) are intended for modules. Don't use them in applications: DP determines the application health from its [workloads](templates.html#workload-health).
- Metrics of hooks are not collected.
