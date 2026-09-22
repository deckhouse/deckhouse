# node-draining

**Name:** `node-draining`
**Primary resource:** `Node`
**Replaces hook:** `handle_draining.go`

## Purpose

Handles the node drain lifecycle. When a node receives the annotation
`update.node.deckhouse.io/draining`, this controller cordons the node, evicts its
pods (respecting PDBs and timeouts), and records the result.

The eviction runs as **one background task per node** (`internal/task`), under a
context the controller can cancel. A reconcile never sits inside it: the task hands
the node back to the workqueue when it ends.

## Watched Resources

| Resource | Trigger | Filter |
|----------|---------|--------|
| `Node` | Any change (primary) | Only nodes with label `node.deckhouse.io/group` |
| wake channel | An eviction finished | none |

The label filter is a `WithEventFilter` predicate. It does not reach the wake channel:
a raw source is exempt from it by design, which is just as well, since the `Node` that
channel carries has a name and nothing else.

## Reconciliation Logic

```
Node changed, or an eviction finished
  │
  ├─ Node is gone? → clear the metric, cancel the eviction → done
  │
  ├─ No "draining" annotation →
  │    ├─ clear the metric
  │    ├─ an eviction is running? → cancel it and wait for it to stop
  │    ├─ "drained=user" on a schedulable node? → remove the stale marker
  │    └─ done (a cordon with no eviction behind it belongs to updateapproval)
  │
  ├─ "drained=user" while a new drain is requested? → remove it, so the new
  │     drain's own result is what the node ends up carrying
  │
  ├─ The eviction has finished →
  │    ├─ succeeded → clear the metric
  │    ├─ hit its own deadline → event + gauge, recorded as drained anyway
  │    ├─ failed otherwise → event + gauge, return the error (retried)
  │    └─ remove "draining", set "drained=<source>"
  │
  └─ Otherwise →
       ├─ node still schedulable? → cordon it → done, the cordon's own event
       │   brings us back (pods must stop arriving before anything empties the node)
       └─ already cordoned → start the eviction
```

Every branch edits the node in memory; a single deferred patch at the end of the
reconcile is the only write.

## One Task Per Node

`internal/task.Manager` keys tasks by node name, so a node never runs two background
operations at once — a second one is refused with `ErrExists`. A finished task is kept
until its result is collected, which is what lets the reconcile decide the outcome.
Cancelling waits for the goroutine to return before the caller does anything else, so
nothing races an eviction still in flight.

The registry lives in memory. A controller restart therefore forgets that an eviction
was running, and a request withdrawn while the controller is down goes unnoticed.

## Drain Timeout Resolution

1. Read the `node.deckhouse.io/group` label from the node
2. Fetch the NodeGroup object
3. Use `spec.nodeDrainTimeoutSecond` if set, otherwise 10 minutes

Resolved in `startDrain` and passed into the task, which turns it into the eviction's
own deadline.

## Key Annotations

| Annotation | Meaning |
|------------|---------|
| `update.node.deckhouse.io/draining` | Drain requested (value = source, e.g. "bashible"; an empty value means "bashible"). **Removing it cancels an eviction in flight** |
| `update.node.deckhouse.io/drained` | Drain completed (value = source). `drained=user` marks a hand drain and is cleaned up in two places: off a schedulable node with no request, and before a new drain starts |

## Events

| Reason | When |
|--------|------|
| `DrainSucceeded` | The eviction ended and the annotations were flipped |
| `DrainFailed` | The eviction failed, or ran out of its timeout |
| `DrainCancelled` | The request was withdrawn and an eviction in flight was stopped |

## Files

- `controller.go` — reconciler and the four situations a node can be in
- `drainer.go` — the background eviction: task registry, wake channel, `kubedrain` call
- `metrics.go` — the `d8_node_draining` failure gauge
- `../../task/manager.go` — one background task per subject, with cancellation
