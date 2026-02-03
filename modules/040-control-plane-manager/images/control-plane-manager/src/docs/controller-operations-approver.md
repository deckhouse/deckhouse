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
   - Calculated from total quorum membership: `masters + arbiters`
2. `KubeAPIServer` (limit = `max(1, masters-1)`)
3. `KubeControllerManager`, `KubeScheduler` (same limit as stage 2)
   - Workload components run exclusively on master nodes

Rules:

- at most 1 approved in-flight operation per `(component, node)`
- unapproved operations are sorted by:
- stage order
- then resource name (stable deterministic tie-break)
- next stage cannot start while any earlier stage has approved in-flight operations

## Reconciliation Logic

1. Query node topology to determine:
   - Count of master nodes (labeled with `node.deckhouse.io/control-plane=""`)
   - Count of arbiter nodes (labeled with `node.deckhouse.io/etcd-arbiter=""`)
2. List all `ControlPlaneOperation` objects.
3. Split operations into:
   - approved and non-terminal (already occupy slots)
   - unapproved (approval queue)
4. Seed stage counters from approved non-terminal operations.
5. Iterate queue and try to reserve slot for each operation.
6. Patch `spec.approved=true` when reservation succeeds.

## Logic Basis

- **Node topology awareness**: Master and arbiter nodes are counted separately:
  - Etcd limit includes both masters and arbiters (full quorum for etcd safety)
  - Workload component limits use only master node count (apiserver, controller-manager, scheduler run on masters only)
- **Safety first for etcd**: Strict single-flight to maintain quorum consensus safety.
- **Control-plane workload rollout**: Allow `N-1` parallel updates (where N = master count), keep at least one node not updating in multi-node setups.
- **Determinism**: Stable sort and explicit stage graph.

