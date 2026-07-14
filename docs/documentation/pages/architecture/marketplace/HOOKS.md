---
title: Hooks
permalink: en/architecture/marketplace/hooks.html
description: "Writing Go hooks for Deckhouse Kubernetes Platform Marketplace Applications using ApplicationHookInput. Instance-scoped ObjectPatcher and settings validation hooks."
---

Application hooks are written in Go using the same module-sdk as Deckhouse Kubernetes Platform (DKP) module hooks. The key difference is that Application hooks use `ApplicationHookInput` instead of `HookInput`, which adds Application-specific capabilities and enforces namespace isolation.

## ApplicationHookInput

`ApplicationHookInput` exposes all the standard hook input methods plus:

- **`Instance()`** — returns metadata about the Application instance, including its name and namespace. Use this to scope operations to the correct namespace.
- **`ObjectPatcher()`** — returns a namespace-scoped patcher. Create and patch operations are limited to the namespace of the Application instance. Hooks cannot create or modify cluster-wide resources.

## Example: basic hook

```go
package main

import (
    "context"

    "github.com/deckhouse/module-sdk/pkg"
    applicationhook "github.com/deckhouse/module-sdk/pkg/app-hook"
)

func main() {
    applicationhook.Run(onSync)
}

func onSync(ctx context.Context, input applicationhook.ApplicationHookInput) error {
    instance := input.Instance()
    // instance.Name — name of the Application resource
    // instance.Namespace — namespace where the Application is installed

    patcher := input.ObjectPatcher()
    // patcher only operates within instance.Namespace

    return nil
}
```

## Settings validation hook

Applications can include a settings validation hook that runs before DKP applies changes. This is used when OpenAPI schema validation is insufficient — for example, to check business logic constraints across multiple settings fields.

The hook implements a `Check` function with the signature:

```go
func Check(_ context.Context, input settingscheck.Input) settingscheck.Result
```

`settingscheck.Input` provides access to the `Application.spec.settings` values.

`settingscheck.Result` is one of:

- `settingscheck.Allow(warnings...)` — settings are valid; optionally attach warning messages.
- `settingscheck.Reject(reason)` — settings are invalid; the Application resource will not be applied.

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

## Namespace isolation guarantee

The `ObjectPatcher` returned by `ApplicationHookInput` enforces that:

- All `Create`, `Patch`, and `Update` calls target the Application's own namespace.
- Calls targeting other namespaces or cluster-wide resources are rejected at runtime.

This is an architectural enforcement, not just a convention. Hooks that need to read (not write) cluster-wide resources can still use `d8 k` or the Kubernetes API client directly, but writes outside the namespace are blocked.
