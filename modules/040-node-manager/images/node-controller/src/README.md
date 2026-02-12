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

### Status Reconciliation Flow

```
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────────┐
│ NodeGroup│  │   Node   │  │ Machine  │  │ MachineDeployment │
│  (watch) │  │  (watch) │  │  (watch) │  │     (watch)       │
└────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬──────────┘
     │             │             │                  │
     └─────────────┴─────────────┴──────────────────┘
                           │
                           ▼
          ┌────────────────────────────────┐
          │  NodeGroupStatusReconciler     │
          │                                │
          │  For each NodeGroup:           │
          │  ┌───────────────────────────┐ │
          │  │ 1. Count nodes by label   │ │
          │  │    node.deckhouse.io/group │ │
          │  │                           │ │
          │  │ 2. Count ready nodes      │ │
          │  │    (condition Ready=True)  │ │
          │  │                           │ │
          │  │ 3. Count upToDate nodes   │ │
          │  │    (checksum matches NG)   │ │
          │  │                           │ │
          │  │ 4. For CloudEphemeral:    │ │
          │  │    ├─ machines count       │ │
          │  │    │  (MCM Machine CRD)    │ │
          │  │    ├─ desired from         │ │
          │  │    │  MachineDeployment    │ │
          │  │    └─ min/max from         │ │
          │  │       cloudInstances spec  │ │
          │  │                           │ │
          │  │ 5. For non-CloudEphemeral:│ │
          │  │    instances = nodes count │ │
          │  │    desired = nodes count   │ │
          │  │                           │ │
          │  │ 6. Build conditions:      │ │
          │  │    ├─ Ready               │ │
          │  │    ├─ Updating            │ │
          │  │    ├─ WaitingForDisrupt.  │ │
          │  │    ├─ Error               │ │
          │  │    ├─ Scaling (Ephemeral) │ │
          │  │    └─ Frozen (Ephemeral)  │ │
          │  │                           │ │
          │  │ 7. Build conditionSummary │ │
          │  │                           │ │
          │  │ 8. Merge-patch status     │ │
          │  │    (preserves other       │ │
          │  │     controller fields)    │ │
          │  └───────────────────────────┘ │
          └────────────────────────────────┘
                           │
                           ▼
                  ┌──────────────┐
                  │  NodeGroup   │
                  │  .status     │
                  │  (patched)   │
                  └──────────────┘
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

## Project Structure

```
node-controller/
├── api/deckhouse.io/
│   ├── v1/                         # Hub version (storage)
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go      # Type definitions with all fields
│   │   ├── nodegroup_conversion.go # Hub() marker
│   │   └── zz_generated.deepcopy.go
│   ├── v1alpha1/                   # Spoke version
│   │   ├── doc.go
│   │   ├── groupversion_info.go
│   │   ├── nodegroup_types.go
│   │   ├── nodegroup_conversion.go # ConvertTo/ConvertFrom
│   │   ├── conversion.go          # Custom field mappings
│   │   └── zz_generated.deepcopy.go
│   └── v1alpha2/                   # Spoke version (+ NotManaged, ContainerdV2)
│       └── ...
├── cmd/
│   └── main.go                     # Entry point, manager setup
├── internal/
│   ├── controller/
│   │   ├── register_controller.go          # Auto-registration via init()
│   │   └── nodegroupstatus/                # Status reconciler package
│   │       ├── controller.go               # Main reconciler logic
│   │       ├── controller_test.go          # Unit tests
│   │       └── README.md                   # Package documentation
│   └── webhook/
│       ├── nodegroup_webhook.go             # Validation webhook (17 checks)
│       └── nodegroup_conversion_handler.go  # Conversion webhook
├── docs/
│   ├── conversion-migration.md
│   └── hooks-migration.md
├── hack/
│   └── boilerplate.go.txt
├── go.mod
├── go.sum
├── Makefile
└── PROJECT
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
