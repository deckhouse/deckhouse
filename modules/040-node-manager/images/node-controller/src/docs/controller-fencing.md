# node-fencing

**Name:** `node-fencing`
**Primary resource:** `Node`
**Replaces hook:** `fencing_controller.go`

## Purpose

Implements node fencing — when a node becomes unresponsive (its Lease expires),
the controller force-deletes all pods from that node and optionally deletes
the Node object itself. This prevents workloads from being stuck on dead nodes.

## Watched Resources

| Resource | Trigger | Filter |
|----------|---------|--------|
| `Node` | Any change (primary) | Only nodes with label `node-manager.deckhouse.io/fencing-enabled` |

## Setup

Registers a field indexer on `Pod.spec.nodeName` to efficiently list pods
by node name via the shared cache.

## Reconciliation Logic

```
Node changed
  │
  ├─ Missing "fencing-enabled" label? → skip
  │
  ├─ Has maintenance annotation? → requeue after 1 min
  │   (disruption-approved, approved, fencing-disable)
  │
  ├─ Lease not found? → requeue after 1 min
  │
  ├─ Lease renewed within 60s? → requeue after 1 min (node is alive)
  │
  └─ Lease expired (>60s since renewTime) →
       ├─ List all pods on node (via field index)
       ├─ Force-delete all pods (gracePeriod=0)
       │
       ├─ shouldDeleteNode? (fencingMode != Notify AND nodeType != Static/CloudStatic)
       │   └─ YES → Delete Node object
       │
       └─ NO → Log "pods deleted, node preserved"
```

## Maintenance Annotations (skip fencing)

| Annotation |
|------------|
| `update.node.deckhouse.io/disruption-approved` |
| `update.node.deckhouse.io/approved` |
| `node-manager.deckhouse.io/fencing-disable` |

## Key Labels

| Label | Purpose |
|-------|---------|
| `node-manager.deckhouse.io/fencing-enabled` | Enables fencing for this node |
| `node-manager.deckhouse.io/fencing-mode` | `Notify` = only delete pods, don't delete node |
| `node.deckhouse.io/type` | `Static`/`CloudStatic` nodes are never deleted |

## Constants

| Name | Value |
|------|-------|
| Fencing timeout | 60 seconds |
| Requeue interval | 1 minute |
| Lease namespace | `kube-node-lease` |

## Files

- `controller.go` — reconciler, pod deletion, node deletion logic
