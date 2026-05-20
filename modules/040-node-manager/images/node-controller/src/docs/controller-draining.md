# node-draining

**Name:** `node-draining`
**Primary resource:** `Node`
**Replaces hook:** `handle_draining.go`

## Purpose

Handles the node drain lifecycle. When a node receives the annotation
`update.node.deckhouse.io/draining`, this controller cordons the node,
evicts all pods (respecting PDBs and timeouts), and marks the node as drained.

## Watched Resources

| Resource | Trigger | Filter |
|----------|---------|--------|
| `Node` | Any change (primary) | Only nodes with label `node.deckhouse.io/group` |

The event filter ensures only managed nodes (those assigned to a NodeGroup)
trigger reconciliation.

## Reconciliation Logic

```
Node changed
  │
  ├─ No "draining" and no "drained" annotation? → skip
  │
  ├─ No "draining" but "drained=user" and node is schedulable?
  │   └─ Remove stale "drained" annotation → done
  │
  ├─ No "draining" annotation? → skip
  │
  └─ Has "draining" annotation →
       ├─ Remove existing "drained=user" annotation if present
       ├─ Resolve drain timeout from NodeGroup.spec.nodeDrainTimeoutSecond
       │   (default: 10 minutes)
       ├─ Cordon node (set spec.unschedulable=true)
       ├─ Drain: evict all pods (parallel, skip DaemonSet + mirror pods)
       │   ├─ Respects PDB (retries on TooManyRequests)
       │   ├─ Waits for pod deletion
       │   └─ On timeout: marks as drained anyway
       ├─ Remove "draining" annotation
       └─ Set "drained=<source>" annotation
```

## Drain Timeout Resolution

1. Read `node.deckhouse.io/group` label from the node
2. Fetch the NodeGroup object
3. Use `spec.nodeDrainTimeoutSecond` if set, otherwise 10 minutes

## Key Annotations

| Annotation | Meaning |
|------------|---------|
| `update.node.deckhouse.io/draining` | Drain requested (value = source, e.g. "bashible") |
| `update.node.deckhouse.io/drained` | Drain completed (value = source) |

## Files

- `controller.go` — reconciler, cordon, timeout resolution
- `drain.go` — pod eviction logic (parallel eviction, PDB retry, deletion wait)
