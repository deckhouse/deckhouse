# bashible-cleanup

**Name:** `bashible-cleanup`
**Primary resource:** `Node`
**Replaces hook:** `remove_bashible_completed_labels_and_taints.go`

## Purpose

Removes initialization artifacts from nodes after bashible completes its first run.
When bashible finishes configuring a node, it sets the label
`node.deckhouse.io/bashible-first-run-finished`. This controller detects that label
and cleans up both the label and the uninitialized taint so the node becomes
fully schedulable.

## Watched Resources

| Resource | Trigger |
|----------|---------|
| `Node` | Any change (primary resource) |

No additional watches (`SetupWatches` is empty) — the controller reacts only
to changes on the Node object itself.

## Reconciliation Logic

```
Node changed
  │
  ├─ Node not found? → done (no-op)
  │
  ├─ Missing label "bashible-first-run-finished"? → skip
  │
  └─ Has label →
       ├─ Remove label "node.deckhouse.io/bashible-first-run-finished"
       ├─ Remove taint "node.deckhouse.io/bashible-uninitialized" (if present)
       └─ Patch node (MergeFrom)
```

## Key Constants

| Key | Value |
|-----|-------|
| Label | `node.deckhouse.io/bashible-first-run-finished` |
| Taint | `node.deckhouse.io/bashible-uninitialized` |

## Files

- `controller.go` — reconciler, single file
