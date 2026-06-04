# Node Controller

Standalone Kubernetes controller for managing NodeGroup resources in Deckhouse.
Replaces several shell/Go hooks from the `040-node-manager` module with a native controller-runtime based controller.

## What it replaces

| Original hook | Replaced by | Status |
|---|---|---|
| `hooks/node_group.py` (conversion webhook) | `internal/webhook/nodegroup_conversion_handler.go`
| `hooks/node_group` (validation webhook) | `internal/webhook/nodegroup_webhook.go`

## Components

### Validation Webhook

17 validations, all ported from the original bash hook `hooks/node_group`.
Only validates things that CRD OpenAPI schema cannot:

| # | Validation | Type |
|---|---|---|
| 1 | Cloud cluster name length (prefix + name ≤ 42) | CREATE |
| 2 | nodeType immutability | UPDATE |
| 3 | minPerZone ≤ maxPerZone | CREATE/UPDATE |
| 4 | maxPods warning (IP exhaustion) | CREATE/UPDATE |
| 5 | Unknown zone (from provider config) | CREATE/UPDATE |
| 6 | Docker CRI forbidden | CREATE/UPDATE |
| 7 | CRI config must match type | CREATE/UPDATE |
| 8 | CRI change on master with <3 endpoints (warning) | UPDATE |
| 9 | Taints not in customTolerationKeys | CREATE/UPDATE |
| 10 | RollingUpdate only for CloudEphemeral | CREATE/UPDATE |
| 11 | staticInstances.labelSelector immutability | UPDATE |
| 12 | Duplicate taints (same key+effect) | CREATE/UPDATE |
| 13 | topologyManager requires resourceReservation | CREATE/UPDATE |
| 14 | topologyManager + Static mode requires cpu | CREATE/UPDATE |
| 15 | CRI change with custom containerd nodes | UPDATE |
| 16 | ContainerdV2 with unsupported nodes | UPDATE |
| 17 | memorySwap requires cgroup v2 | UPDATE |

Validations handled by CRD and NOT duplicated in webhook:
- Name format (pattern), name length (maxLength)
- nodeType enum values
- CloudEphemeral requires cloudInstances (oneOf)
- Static must not have cloudInstances (oneOf)
- cloudInstances requires classReference (required)
- CRI type enum values

### Conversion Webhook

Custom conversion handler (not using standard `conversion.NewWebhookHandler()`) because it needs cluster state access to determine `CloudPermanent` vs `CloudStatic` when converting `Hybrid` nodeType from spoke versions.

Reads provider config from Secret `kube-system/d8-provider-cluster-configuration` to check if a NodeGroup name is listed in provider's node groups (→ CloudPermanent) or not (→ CloudStatic).

## Request Flow

### Validation Flow

```
kubectl apply NodeGroup
        │
        ▼
┌─────────────────┐
│  kube-apiserver  │
│  (CRD OpenAPI)   │──── name pattern, maxLength, nodeType enum,
│                  │     cloudInstances oneOf, classReference required
└────────┬────────┘
         │ (if CRD passes)
         ▼
┌──────────────────────────────────────────────────────┐
│  ValidatingWebhook: node-controller :9443             │
│  POST /validate-deckhouse-io-v1-nodegroup             │
│                                                      │
│  ┌─────────────────────────────────────────────────┐ │
│  │ 1. Decode AdmissionRequest (new + old object)   │ │
│  │ 2. Load cluster state:                          │ │
│  │    ├─ Secret d8-cluster-configuration           │ │
│  │    │  (clusterType, prefix, defaultCRI,         │ │
│  │    │   podSubnetNodeCIDRPrefix)                  │ │
│  │    ├─ Secret d8-provider-cluster-configuration  │ │
│  │    │  (zones from discovery data)               │ │
│  │    ├─ ModuleConfig "global"                     │ │
│  │    │  (customTolerationKeys)                    │ │
│  │    ├─ Endpoints "kubernetes"                    │ │
│  │    │  (apiserver endpoint count)                │ │
│  │    └─ Nodes with specific labels                │ │
│  │       (custom containerd, v2-unsupported)       │ │
│  │ 3. Run 17 validations                           │ │
│  │ 4. Return Allowed/Denied/Warnings               │ │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
         │
         ▼
    ┌──────────┐   ┌──────────┐
    │ Allowed  │   │  Denied  │
    │ (+ warn) │   │ (+ msg)  │
    └──────────┘   └──────────┘
```

### Conversion Flow

```
kubectl get nodegroups.v1alpha2.deckhouse.io
        │
        ▼
┌─────────────────┐
│  kube-apiserver  │  storage version: v1
│                  │  requested: v1alpha2
│  "need to convert│  
│   v1 → v1alpha2" │
└────────┬────────┘
         │
         ▼
┌──────────────────────────────────────────────────────┐
│  ConversionWebhook: node-controller :9443             │
│  POST /convert                                        │
│                                                      │
│  ┌─────────────────────────────────────────────────┐ │
│  │ 1. Parse ConversionReview                       │ │
│  │ 2. For each object:                             │ │
│  │    ├─ Determine source version                  │ │
│  │    ├─ Convert to Hub (v1) if needed             │ │
│  │    └─ Convert from Hub to target version        │ │
│  │                                                 │ │
│  │ Spoke → Hub (v1alpha1/v1alpha2 → v1):           │ │
│  │    Cloud  → CloudEphemeral                      │ │
│  │    Static → Static                              │ │
│  │    Hybrid → ? (need cluster state!)             │ │
│  │      ├─ Read Secret d8-provider-cluster-config  │ │
│  │      ├─ name in provider nodeGroups?            │ │
│  │      │   YES → CloudPermanent                   │ │
│  │      │   NO  → CloudStatic                      │ │
│  │      └─ name == "master" → CloudPermanent       │ │
│  │                                                 │ │
│  │ Hub → Spoke (v1 → v1alpha1/v1alpha2):           │ │
│  │    CloudEphemeral → Cloud                       │ │
│  │    CloudPermanent → Hybrid                      │ │
│  │    CloudStatic    → Hybrid                      │ │
│  │    Static         → Static                      │ │
│  │                                                 │ │
│  │ 3. Return converted objects                     │ │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────┐
│  kube-apiserver  │
│  returns v1alpha2│
│  to client       │
└─────────────────┘
```

## API Versions

| Version | Role | nodeType values |
|---|---|---|
| `v1` | Hub (storage) | CloudEphemeral, CloudPermanent, CloudStatic, Static |
| `v1alpha2` | Spoke | Cloud, Static, Hybrid (+ NotManaged CRI, ContainerdV2) |
| `v1alpha1` | Spoke | Cloud, Static, Hybrid |

### Conversion Mapping

```
v1alpha1/v1alpha2 → v1:
  Cloud  → CloudEphemeral (if has cloudInstances)
  Static → Static
  Hybrid → CloudPermanent (if name in provider config) or CloudStatic

v1 → v1alpha1/v1alpha2:
  CloudEphemeral → Cloud
  CloudPermanent → Hybrid
  CloudStatic    → Hybrid
  Static         → Static
```

## How Shell-Operator Works

Shell-Operator is a framework for building Kubernetes operators using scripts or Go functions.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           SHELL-OPERATOR                                     │
│                                                                             │
│  ╔═══════════════════════════════════════════════════════════════════════╗ │
│  ║  STARTUP                                                               ║ │
│  ║                                                                       ║ │
│  ║  1. Reads all hooks from /hooks/                                      ║ │
│  ║  2. For each hook, parses bindings (which resources to watch)         ║ │
│  ║  3. For each binding:                                                 ║ │
│  ║     • Makes List request to API Server (gets all objects)             ║ │
│  ║     • Applies FilterFunc (extracts only needed fields)                ║ │
│  ║     • Saves result in cache (separate cache per binding)              ║ │
│  ║     • Opens Watch connection (for receiving events)                   ║ │
│  ╚═══════════════════════════════════════════════════════════════════════╝ │
│                                                                             │
│  ╔═══════════════════════════════════════════════════════════════════════╗ │
│  ║  RUNTIME                                                               ║ │
│  ║                                                                       ║ │
│  ║      Watch ◄═══════════════════════════════════════ API Server        ║ │
│  ║        │                                                              ║ │
│  ║        │  Event: "NodeGroup system changed"                           ║ │
│  ║        │  • Streaming connection (not polling)                        ║ │
│  ║        │  • API Server pushes events                                  ║ │
│  ║        ▼                                                              ║ │
│  ║   ┌─────────────────────────────────────────────────────────────┐    ║ │
│  ║   │  FilterFunc(object)                                          │    ║ │
│  ║   │  • Extracts only needed fields                              │    ║ │
│  ║   │  • NodeGroup ~5KB → statusNodeGroup ~100 bytes              │    ║ │
│  ║   └─────────────────────────────────────────────────────────────┘    ║ │
│  ║        │                                                              ║ │
│  ║        ▼                                                              ║ │
│  ║   cachedObjects[binding] = FilterFunc result                          ║ │
│  ║        │                                                              ║ │
│  ║        ▼                                                              ║ │
│  ║   Queue.Add(event)                                                    ║ │
│  ╚═══════════════════════════════════════════════════════════════════════╝ │
│                                                                             │
│  ╔═══════════════════════════════════════════════════════════════════════╗ │
│  ║  HOOK EXECUTION                                                        ║ │
│  ║                                                                       ║ │
│  ║   Build snapshots from cache (NO API requests):                       ║ │
│  ║   snapshots = {                                                       ║ │
│  ║     "ngs":   cachedObjects["ngs"],                                    ║ │
│  ║     "nodes": cachedObjects["nodes"],                                  ║ │
│  ║   }                                                                   ║ │
│  ║        │                                                              ║ │
│  ║        ▼                                                              ║ │
│  ║   hook.Run(input)                                                     ║ │
│  ║   • input.Snapshots contains all data                                 ║ │
│  ║   • Hook reads: input.Snapshots.Get("ngs")                            ║ │
│  ║   • Hook creates patches: input.PatchCollector.Patch(...)             ║ │
│  ║        │                                                              ║ │
│  ║        ▼                                                              ║ │
│  ║   Apply patches → PATCH to API Server (only API request)              ║ │
│  ╚═══════════════════════════════════════════════════════════════════════╝ │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## How Node-Controller Works

Node-Controller uses controller-runtime library with native Kubernetes patterns.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           NODE-CONTROLLER                                    │
│                                                                             │
│  ╔═══════════════════════════════════════════════════════════════════════╗ │
│  ║  STARTUP                                                               ║ │
│  ║                                                                       ║ │
│  ║  1. Controller-manager starts                                         ║ │
│  ║  2. Creates Shared Informer Cache:                                    ║ │
│  ║     • Makes List for each resource type                               ║ │
│  ║     • Saves FULL objects (no FilterFunc)                              ║ │
│  ║     • Opens Watch connections                                         ║ │
│  ║  3. Starts Webhook Server on :9443                                    ║ │
│  ║  4. Registers Reconcilers                                             ║ │
│  ║                                                                       ║ │
│  ║  Key: ONE shared cache for all components                             ║ │
│  ╚═══════════════════════════════════════════════════════════════════════╝ │
│                                                                             │
│  ╔═══════════════════════════════════════════════════════════════════════╗ │
│  ║  SHARED INFORMER CACHE                                                 ║ │
│  ║                                                                       ║ │
│  ║   NodeGroup:         [master, system, worker]                         ║ │
│  ║   Node:              [node-1, node-2, node-3, ...]                    ║ │
│  ║   Machine:           [machine-1, machine-2, ...]                      ║ │
│  ║   MachineDeployment: [md-system, md-worker, ...]                      ║ │
│  ║                                                                       ║ │
│  ║   • Stores FULL objects (all fields available)                        ║ │
│  ║   • Updated automatically via Watch                                   ║ │
│  ║   • All components read from ONE cache                                ║ │
│  ║                         ▲                                             ║ │
│  ║                         │ Watch (streaming)                           ║ │
│  ║                    API Server                                         ║ │
│  ╚═══════════════════════════════════════════════════════════════════════╝ │
│                                                                             │
│            ┌────────────────────┼────────────────────┐                     │
│            ▼                    ▼                    ▼                     │
│  ╔═════════════════╗ ╔═════════════════╗ ╔═════════════════╗              │
│  ║   VALIDATING    ║ ║   CONVERSION    ║ ║   RECONCILER    ║              │
│  ║    WEBHOOK      ║ ║    WEBHOOK      ║ ║                 ║              │
│  ║                 ║ ║                 ║ ║                 ║              │
│  ║ Called BEFORE   ║ ║ Called for      ║ ║ Called when     ║              │
│  ║ saving to etcd  ║ ║ version convert ║ ║ resources change║              │
│  ║                 ║ ║                 ║ ║                 ║              │
│  ║ r.Get() → CACHE ║ ║ r.Get() → CACHE ║ ║ r.Get() → CACHE ║              │
│  ╚═════════════════╝ ╚═════════════════╝ ╚═════════════════╝              │
│                                                                             │
│  ╔═══════════════════════════════════════════════════════════════════════╗ │
│  ║  RECONCILER                                                            ║ │
│  ║                                                                       ║ │
│  ║   Reconcile(ctx, Request{Name: "system"})                             ║ │
│  ║                                                                       ║ │
│  ║   1. r.Get(ctx, name, &ng)       ──► reads from CACHE                 ║ │
│  ║   2. r.List(ctx, &nodes, ...)    ──► reads from CACHE                 ║ │
│  ║   3. r.List(ctx, &machines, ...) ──► reads from CACHE                 ║ │
│  ║   4. Calculate status                                                 ║ │
│  ║   5. r.Status().Patch()          ──► ONLY API request                 ║ │
│  ║                                                                       ║ │
│  ║   Built-in: rate limiting, backoff, leader election, metrics          ║ │
│  ╚═══════════════════════════════════════════════════════════════════════╝ │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Key Differences

### Cache Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  SHELL-OPERATOR                                                             │
│                                                                             │
│  Hook 1: cache [NodeGroup, Node, Machine, MachineDeployment]                │
│  Hook 2: cache [NodeGroup, Secret, ModuleConfig, Endpoints]                 │
│  Hook 3: cache [NodeGroup, Node]                                            │
│                                                                             │
│  → NodeGroup stored in 3 caches = memory duplication                        │
├─────────────────────────────────────────────────────────────────────────────┤
│  NODE-CONTROLLER                                                            │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    SHARED INFORMER CACHE                             │   │
│  │                                                                     │   │
│  │  NodeGroup ─────┬────► Validating Webhook                           │   │
│  │                 ├────► Conversion Webhook                           │   │
│  │                 └────► Reconciler                                   │   │
│  │                                                                     │   │
│  │  One cache, all components read from it                             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Controller Registration

### Shell-Operator Style

```go
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
    Queue: "/modules/node-manager/update_ngs_statuses",
    Kubernetes: []go_hook.KubernetesConfig{
        {
            Name:       "ngs",
            Kind:       "NodeGroup",
            ApiVersion: "deckhouse.io/v1",
            FilterFunc: updStatusFilterNodeGroup,  // filtering
        },
        {
            Name:          "nodes",
            Kind:          "Node",
            LabelSelector: &metav1.LabelSelector{...},
            FilterFunc:    updStatusFilterNode,
        },
    },
}, handleUpdateNGStatus)
```

### Node-Controller Style

```go
// Step 1: Auto-register controller
var _ = Register("NodeGroupStatus", SetupNodeGroupStatusController)

// Step 2: Setup function
func SetupNodeGroupStatusController(mgr ctrl.Manager) error {
    return (&NodeGroupStatusReconciler{
        Client:   mgr.GetClient(),
        Recorder: mgr.GetEventRecorderFor("node-controller"),
    }).SetupWithManager(mgr)
}

// Step 3: Configure watches
func (r *NodeGroupStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1.NodeGroup{}).                           // primary resource
        Watches(                                        // secondary: Node
            &corev1.Node{},
            handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroup),
            builder.WithPredicates(nodeHasGroupLabel),
        ).
        Watches(                                        // secondary: Machine
            &unstructured.Unstructured{...MCMMachineGVK},
            handler.EnqueueRequestsFromMapFunc(r.machineToNodeGroup),
        ).
        Watches(                                        // secondary: MachineDeployment
            &unstructured.Unstructured{...MCMMachineDeploymentGVK},
            handler.EnqueueRequestsFromMapFunc(r.mdToNodeGroup),
        ).
        Named("nodegroup-status").
        Complete(r)
}
```

### Registration Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  1. Go loads package → var _ = Register(...) executes                       │
│                                                                             │
│  2. Register() adds to global controllers slice:                            │
│     controllers = [{name: "NodeGroupStatus", setup: SetupFunc}]             │
│                                                                             │
│  3. main() calls controller.SetupAll(mgr, disabled)                         │
│                                                                             │
│  4. SetupAll loops through controllers, calls setup(mgr) for each           │
│                                                                             │
│  5. Setup configures:                                                       │
│     • For(&NodeGroup{})           — primary, Reconcile gets its name        │
│     • Watches(&Node{})            — secondary, mapFunc → primary name       │
│     • Watches(&Machine{})         — secondary, mapFunc → primary name       │
│     • Predicate                   — filters events (not objects)            │
│                                                                             │
│  6. mgr.Start() runs informers and reconcile loop                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Project Structure

```
node-controller/src/
├── api/deckhouse.io/
│   ├── v1/                                    # Hub version (storage)
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go                 # Type definitions with all fields
│   │   ├── nodegroup_conversion.go            # Hub() marker
│   │   └── zz_generated.deepcopy.go
│   ├── v1alpha1/                              # Spoke version
│   │   ├── doc.go
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go
│   │   ├── nodegroup_conversion.go            # ConvertTo/ConvertFrom
│   │   ├── conversion.go                      # Custom field mappings
│   │   └── zz_generated.deepcopy.go
│   └── v1alpha2/                              # Spoke version (+ NotManaged, ContainerdV2)
│       ├── doc.go
│       ├── groupversion_info.go
│       ├── nodegroup_types.go
│       ├── nodegroup_conversion.go
│       └── zz_generated.deepcopy.go
├── cmd/
│   └── main.go                                # Entry point, manager setup
├── internal/
│   ├── controller/
│   │   ├── register_controller.go             # Registry + SetupAll for auto-registration
│   │   ├── nodegroup_status_controller.go     # Status reconciler (init() auto-registers)
│   │   └── nodegroup_status_controller_test.go
│   └── webhook/
│       ├── nodegroup_webhook.go               # Validation webhook (17 checks)
│       ├── nodegroup_webhook_test.go
│       ├── nodegroup_conversion_handler.go    # Conversion webhook
│       └── nodegroup_conversion_handler_test.go
├── docs/
│   └── hooks-migration.md
├── hack/
│   └── boilerplate.go.txt
├── go.mod
├── go.sum
├── Makefile
├── PROJECT
└── README.md
```

## Building

```bash
# Generate DeepCopy methods
make generate

# Build binary
make build

# Build Docker image
make docker-build IMG=your-registry/node-controller:tag

# Run locally (requires kubeconfig)
make run
```

## Dev Builds with Werf

```bash
# Build specific image
bin/werf build node-manager/node-controller \
  --repo <REGISTRY>/deckhouse \
  --save-build-report=true \
  --build-report-path images_tags_werf.json

# Get image name from report
jq -r '.Images."node-manager/node-controller".DockerImageName' images_tags_werf.json

# Deploy to cluster
kubectl set image deployment/node-controller-manager \
  -n d8-cloud-instance-manager \
  node-controller=<IMAGE_FROM_JSON>
```

## Testing

### Unit Tests

```bash
go test ./internal/webhook/... -v
go test ./internal/controller/... -v
```

## Deployment

The controller is deployed as part of the `040-node-manager` module:
- Deployment: `d8-cloud-instance-manager/node-controller-manager`
- Webhook certificates: generated by `hooks/generate_node_controller_webhook_certs.go`
- CRD patching: `hooks/nodegroup_crd_conversion_webhook.go` patches the CRD to point conversion webhook to node-controller

## License

Apache License 2.0
