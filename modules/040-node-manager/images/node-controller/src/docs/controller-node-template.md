# node-template

**Name:** `node-template`
**Primary resource:** `Node`
**Replaces hook:** `handle_node_templates.go`

## Purpose

Applies labels, annotations, and taints from `NodeGroup.spec.nodeTemplate`
to all nodes belonging to that NodeGroup. Tracks previously applied templates
via the `node.deckhouse.io/last-applied-node-template` annotation to correctly
remove stale entries when the template changes.

## Watched Resources

| Resource | Trigger | MapFunc |
|----------|---------|---------|
| `Node` | Any change (primary) | Reconcile that specific node |
| `NodeGroup` | Any change | Maps to a synthetic request `__all__` → reconcile all nodes |

## Reconciliation Logic

### Single Node (triggered by Node change)

```
Node changed
  │
  ├─ No "node.deckhouse.io/group" label? → skip
  ├─ NodeGroup not found? → skip
  └─ reconcileNode(node, ng)
```

### All Nodes (triggered by NodeGroup change)

```
NodeGroup changed → enqueue "__all__"
  │
  ├─ List all Nodes
  ├─ List all NodeGroups (build map by name)
  ├─ Sync metrics (unmanaged nodes, missing master taints)
  └─ For each node with a nodeGroup label:
       └─ reconcileNode(node, ng)
```

### reconcileNode

```
1. DeepCopy node → base + working

2. CloudEphemeral node:
   ├─ Fix cloud taints (merge template taints, remove uninitialized taint)
   ├─ If CAPI node → apply full template
   └─ If non-CAPI → taints only

3. Other nodeTypes → apply full template

4. Master node:
   ├─ Set role labels (node-role.kubernetes.io/control-plane, master)
   └─ Fix master taints (remove deprecated master taint if control-plane exists)

5. Non-CloudEphemeral → set scale-down-disabled=true annotation

6. Compare base vs working → if changed, Patch(MergeFrom)
```

### applyNodeTemplate (three-way merge)

For labels, annotations, and taints the controller performs a three-way merge:
- **actual** — current state on the node
- **desired** — from `NodeGroup.spec.nodeTemplate`
- **lastApplied** — from `node.deckhouse.io/last-applied-node-template` annotation

This allows the controller to:
- Add new template entries
- Update changed template entries
- Remove entries that were in lastApplied but no longer in desired
- Preserve entries set by users/other controllers

## Key Annotations

| Annotation | Purpose |
|------------|---------|
| `node.deckhouse.io/last-applied-node-template` | JSON of previously applied template |
| `cluster-autoscaler.kubernetes.io/scale-down-disabled` | Set for non-CloudEphemeral nodes |

## Metrics

| Metric | Description |
|--------|-------------|
| Unmanaged nodes count | Nodes without `node.deckhouse.io/group` label |
| Missing master taint | Master nodes missing `node-role.kubernetes.io/control-plane` taint |

## Files

- `controller.go` — reconciler, single/all node modes
- `reconcile_service.go` — `reconcileNode()` orchestration
- `template_service.go` — `applyNodeTemplate()` three-way merge
- `taints.go` — taint merge, master taint fix, cloud taint fix
- `helpers.go` — utility functions, `nodeChanged()`, `shouldDisableScaleDown()`
- `constants.go` — label/annotation keys
- `metrics.go` — Prometheus metrics
