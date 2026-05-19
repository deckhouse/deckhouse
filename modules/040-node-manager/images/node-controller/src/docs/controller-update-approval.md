# nodegroup-update-approval

**Name:** `nodegroup-update-approval`
**Primary resource:** `NodeGroup`
**Replaces hook:** `update_approval.go`

## Purpose

Manages the node update lifecycle: approving updates, handling disruptions,
managing drain-before-disruption, and cleaning up after successful updates.
Operates per-NodeGroup with concurrency control based on `spec.update.maxConcurrent`.

## Watched Resources

| Resource | Trigger | MapFunc |
|----------|---------|---------|
| `NodeGroup` | Any change (primary) | — |
| `Node` | Any change | `node.deckhouse.io/group` → NodeGroup name |
| `Secret` (configuration-checksums) | Change | Enqueue all NodeGroups |

Node events are filtered to only include nodes with `node.deckhouse.io/group` label.

## Reconciliation Logic

The reconciler runs three phases sequentially, stopping at the first phase
that makes a change:

```
Reconcile(NodeGroup)
  │
  ├─ Get NodeGroup, get configuration checksums from Secret
  ├─ List nodes for this NodeGroup
  ├─ Build NodeInfo for each node (annotations → flags)
  ├─ Export per-node metrics
  │
  ├─ Phase 1: ProcessUpdatedNodes
  │   └─ For nodes that are approved + checksum matches + ready:
  │       remove all update annotations, uncordon if drained → "UpToDate"
  │
  ├─ Phase 2: ApproveDisruptions
  │   └─ For nodes that are approved + need disruption:
  │       ├─ Check approval mode (Manual/Automatic/RollingUpdate)
  │       ├─ Check disruption windows
  │       ├─ RollingUpdate → delete Instance
  │       ├─ No drain needed or already drained → approve disruption
  │       └─ Needs drain → set "draining=bashible" annotation
  │
  └─ Phase 3: ApproveUpdates
      └─ For nodes waiting-for-approval:
          ├─ Calculate max concurrent from spec.update.maxConcurrent
          ├─ Count currently approved nodes
          ├─ Prefer ready nodes first, then not-ready
          └─ Set "approved" annotation, remove "waiting-for-approval"
```

## Update Annotations

| Annotation | Meaning |
|------------|---------|
| `update.node.deckhouse.io/waiting-for-approval` | Node requests update approval |
| `update.node.deckhouse.io/approved` | Update approved |
| `update.node.deckhouse.io/disruption-required` | Disruptive operation needed |
| `update.node.deckhouse.io/disruption-approved` | Disruption approved |
| `update.node.deckhouse.io/draining` | Drain in progress |
| `update.node.deckhouse.io/drained` | Drain completed |

## Disruption Approval Modes

| Mode | Behavior |
|------|----------|
| `Manual` | Never auto-approve disruptions |
| `Automatic` | Auto-approve within configured windows |
| `RollingUpdate` | Delete instance and recreate (CloudEphemeral only) |

## Concurrency Control

`spec.update.maxConcurrent` (IntOrString) limits how many nodes in a NodeGroup
can be updated simultaneously. Supports absolute numbers and percentages.

## Drain Decision

The controller skips draining when:
- Single control-plane node (`master` group with 1 node)
- Deckhouse pod runs on this node and NodeGroup has < 2 ready nodes
- `spec.disruptions.automatic.drainBeforeApproval` is explicitly `false`

## Sub-packages

| Package | Purpose |
|---------|---------|
| `common/` | NodeInfo builder, annotation constants, window checking, concurrency calc |
| `engine/` | Processor with three phases (ProcessUpdatedNodes, ApproveDisruptions, ApproveUpdates) |
| `kubeclient/` | Kubernetes API helpers (PatchNode, GetNodes, DeleteInstance, GetChecksums) |
| `metrics/` | Per-node update status metrics |

## Files

- `controller.go` — reconciler, watches, NodeGroup→Node mapping
- `engine/processor.go` — three-phase update logic
- `common/common.go` — NodeInfo, constants, disruption windows, concurrency
- `kubeclient/client.go` — Kubernetes client wrapper
- `metrics/calculator.go` — Prometheus metrics registration and export
