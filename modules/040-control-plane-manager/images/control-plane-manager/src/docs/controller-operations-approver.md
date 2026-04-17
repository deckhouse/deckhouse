# operations-approver

**Name:** `operations_approver_controller`  
**Primary resource:** `ControlPlaneOperation`

## Purpose

Set `spec.approved=true` for queued operations using stage ordering and concurrency limits across control-plane nodes.

## Watched Resources

| Resource | Trigger |
|---|---|
| `ControlPlaneOperation` | create |
| `ControlPlaneOperation` | update when operation becomes terminal |

The controller recomputes approvals for the full operation set on each trigger.

## Approval Model

Approval pipeline stages:

1. `Etcd` (global concurrency limit = `1`)
2. `KubeAPIServer` (limit = `max(1, nodesCount-1)`)
3. `KubeControllerManager`, `KubeScheduler` (same limit as stage 2)

Rules:

- at most 1 approved in-flight operation per `(component, node)`
- unapproved operations are sorted by:
- stage order
- then resource name (stable deterministic tie-break)
- next stage cannot start while any earlier stage has approved in-flight operations

## Reconciliation Logic

1. List all `ControlPlaneNode` objects to get node count.
2. List all `ControlPlaneOperation` objects.
3. Split operations into:
- approved and non-terminal (already occupy slots)
- unapproved (approval queue)
4. Seed stage counters from approved non-terminal operations.
5. Iterate queue and try to reserve slot for each operation.
6. Patch `spec.approved=true` when reservation succeeds.

## Logic Basis

- Safety first for etcd: strict single-flight.
- Control-plane workload rollout: allow `N-1` parallel updates, keep at least one node not updating in multi-node setups.
- Determinism: stable sort and explicit stage graph.

