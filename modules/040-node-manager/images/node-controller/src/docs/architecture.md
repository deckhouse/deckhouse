# Architecture

## Overview

Node-controller is a standalone Kubernetes controller built on `controller-runtime`.
It replaces shell-operator Go hooks that reconcile Node and NodeGroup objects
with native Kubernetes controllers using standard reconciliation loops.

All controllers share a single informer cache, which eliminates memory
duplication inherent in per-hook caches of shell-operator.

## Project Layout

```
node-controller/src/
├── cmd/main.go                        # Entry point, manager setup
├── api/deckhouse.io/
│   ├── v1/                            # Hub (storage) version
│   ├── v1alpha1/                      # Spoke version
│   └── v1alpha2/                      # Spoke version
├── internal/
│   ├── register/                      # Controller registration framework
│   │   ├── register.go                # Global registry + SetupAll
│   │   ├── setup.go                   # Per-controller wiring (client, recorder, watches)
│   │   ├── base.go                    # Base struct with Client + Recorder
│   │   ├── watcher.go                 # Watcher interface wrapping ctrl.Builder
│   │   └── controllers/controllers.go # Blank imports triggering init()
│   ├── controller/
│   │   ├── bashiblecleanup/           # Removes bashible init artifacts from nodes
│   │   ├── draining/                  # Handles node drain lifecycle
│   │   ├── fencing/                   # Fencing of unresponsive nodes
│   │   ├── nodegroup/                 # NodeGroup status reconciliation
│   │   ├── nodetemplate/              # Applies NodeGroup templates to nodes
│   │   ├── staticproviderid/          # Sets providerID on static nodes
│   │   └── updateapproval/            # Update/disruption approval workflow
│   └── webhook/
│       ├── nodegroup_webhook.go       # Validation webhook (17 checks)
│       └── nodegroup_conversion_handler.go  # Conversion webhook v1↔v1alpha*
└── docs/
```

## Controller Registration

Registration uses an `init()` pattern — each controller package calls
`register.RegisterController()` at import time. The main binary imports
all packages via a single blank import of `internal/register/controllers`.

### Flow

```
1. Go loads internal/register/controllers/controllers.go
   └── blank-imports every controller package

2. Each controller's init() runs:
   register.RegisterController("name", &primaryObj{}, &Reconciler{})
   └── appends to global []entry slice

3. main() creates ctrl.Manager and calls:
   register.SetupAll(mgr, disabledControllers, maxConcurrentReconciles)

4. SetupAll iterates entries, for each:
   a. Skips if name is in --disable-controllers flag
   b. Injects Client  (if implements NeedsClient)
   c. Injects Recorder (if implements NeedsRecorder)
   d. Calls Setup(mgr) (if implements NeedsSetup)
   e. Builds controller:
      ctrl.NewControllerManagedBy(mgr).
        Named(name).
        For(primaryObj).
        WithOptions(MaxConcurrentReconciles).
        ... + r.SetupWatches(watcher) ...
        Complete(r)

5. mgr.Start() runs shared informers + reconcile loops
```

### Interfaces

Every controller must implement `register.Reconciler`:

```go
type Reconciler interface {
    SetupWatches(w Watcher)
    Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
}
```

Optional interfaces for dependency injection:

| Interface | Method | Injected |
|-----------|--------|----------|
| `NeedsClient` | `InjectClient(client.Client)` | Shared cache client |
| `NeedsRecorder` | `InjectRecorder(record.EventRecorder)` | Per-controller event recorder |
| `NeedsSetup` | `Setup(mgr ctrl.Manager) error` | One-time setup (e.g. field indexers) |

Embedding `register.Base` satisfies `NeedsClient` and `NeedsRecorder` automatically.

### Disabling Controllers

Pass `--disable-controllers=name1,name2` to skip specific controllers at startup.
Controller names are the first argument to `RegisterController()`.

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--metrics-bind-address` | `:8080` | Prometheus metrics endpoint |
| `--health-probe-bind-address` | `:8081` | Health/readiness probes |
| `--disable-controllers` | `""` | Comma-separated controllers to skip |
| `--max-concurrent-reconciles` | `1` | Max parallel reconciles per controller |
| `--logging-format` | `text` | `text` or `json` |

## Shared Informer Cache

All controllers and webhooks read from a single shared informer cache.
Reads (`Get`, `List`) hit the cache — no API requests.
Writes (`Patch`, `Update`, `Delete`, `Create`) go directly to the API server.

```
┌──────────────────────────────────────────────────┐
│            SHARED INFORMER CACHE                 │
│                                                  │
│  NodeGroup  ──┬── nodegroup-status controller    │
│               ├── nodegroup-update-approval      │
│               └── node-template controller       │
│                                                  │
│  Node ────────┬── bashible-cleanup               │
│               ├── node-draining                  │
│               ├── node-fencing                   │
│               ├── node-template                  │
│               ├── static-provider-id             │
│               ├── nodegroup-status               │
│               └── nodegroup-update-approval      │
│                                                  │
│  Machine/MD ──┬── nodegroup-status               │
│                                                  │
│  Lease ───────┴── node-fencing                   │
│                                                  │
│  Secret ──────┬── nodegroup-status               │
│               └── nodegroup-update-approval      │
└──────────────────────────────────────────────────┘
```
