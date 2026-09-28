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
  │    ├─ an eviction is running? → cancel it and wait for it to stop
  │    ├─ clear the metric and remove "drain-failed": nothing is being attempted
  │    │    any more
  │    ├─ "drained=user" on a schedulable node? → remove the stale marker
  │    └─ done (a cordon with no eviction behind it belongs to updateapproval)
  │
  ├─ "drained=user" while a new drain is requested? → remove it, so the new
  │     drain's own result is what the node ends up carrying
  │
  ├─ The eviction has finished →
  │    ├─ failed, deadline included → event + "drain-failed=<error>", keep
  │    │    "draining", return the error (the requeue retries it)
  │    └─ succeeded → clear the metric, remove "drain-failed" and "draining",
  │         set "drained=<source>"
  │
  └─ Otherwise →
       ├─ node still schedulable? → cordon it → done, the cordon's own event
       │   brings us back (pods must stop arriving before anything empties the node)
       └─ already cordoned → start the eviction
```

Every branch edits the node in memory; a single deferred patch at the end of the
reconcile is the only write. That same deferred step sets `d8_node_draining` from the
annotations it is about to patch, so a failure an earlier run of this controller left
on the node raises the gauge too. Three branches lower it again — the node is gone, the
request is withdrawn, the drain succeeds — and no other pass does, because a pass that
starts a retry can read the node from a cache older than the marker.

A drain that keeps failing therefore holds its node in `draining` until some attempt
succeeds, which is intended. The node still carries its pods, so the controller must
not write `drained`. `updateapproval` approves a node's disruption only once it carries
`drained`, so the node holds its update slot — one per NodeGroup, since
`spec.update.maxConcurrent` defaults to 1. The group stops there instead of updating a
node that nothing emptied, and `NodeStuckInDraining` fires five minutes into the failure
and keeps firing while the drain keeps failing.

The cause is usually a PodDisruptionBudget that never allows the eviction, or a pod
that never terminates. Once it is gone the next attempt succeeds and the slot is
released.

## One Task Per Node

`internal/task.Manager` keys tasks by node name, so a node never runs two background
operations at once — a second one is refused with `ErrExists`. A finished task is kept
until its result is collected, which is what lets the reconcile decide the outcome.
Cancelling waits for the goroutine to return before the caller does anything else, so
nothing races an eviction still in flight.

The registry lives in memory. A controller restart therefore forgets that an eviction
was running. The request is still on the node, so the first reconcile after the restart
starts a fresh attempt, and a `drain-failed` annotation left by an earlier attempt raises
the gauge again. A request withdrawn while the controller was down leaves nothing to
cancel, so no `DrainCancelled` event is recorded.

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
| `update.node.deckhouse.io/drain-failed` | The last attempt failed (value = the error, flattened to one line and cut to 1 KB, with `… (truncated)` appended when it was cut). Removed by a successful drain and by the withdrawal of the request. A failed drain is **not** recorded as `drained`: it is retried, and `NodeStuckInDraining` fires five minutes into a drain that keeps failing |

## Events

| Reason | When |
|--------|------|
| `DrainSucceeded` | The eviction ended and the annotations were flipped |
| `DrainFailed` | The eviction failed, or ran out of its timeout — one event per attempt, so a drain that keeps failing records a new one on every retry. The event carries the full error, the annotation only its first 1 KB |
| `DrainCancelled` | The request was withdrawn and an eviction in flight was stopped |

## Files

- `controller.go` — reconciler and the four situations a node can be in
- `drainer.go` — the background eviction: task registry, wake channel, `kubedrain` call
- `metrics.go` — the `d8_node_draining` gauge, a view of the `drain-failed` annotation
- `../../task/manager.go` — one background task per subject, with cancellation
