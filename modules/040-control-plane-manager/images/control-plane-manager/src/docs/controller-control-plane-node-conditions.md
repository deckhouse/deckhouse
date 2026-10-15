# control-plane-node-conditions

**Name:** `control-plane-node-conditions-controller`
**Primary resource:** `ControlPlaneNode` (status.conditions only)

## Purpose

Maintain `ControlPlaneNode.status.conditions` — the pod-based readiness of the control plane
components on the node, whether the desired configuration is applied per component, their
per-component summary (shown by `kubectl get cpn`, read by dhctl) and overall certificate health.

Conditions are pointers, not mirrors: the details (step progress, failures, observations) live
in the `ControlPlaneOperation`s and pods referenced by the messages.

## Status Field Ownership

`ControlPlaneNode.status` is written by two node-local controllers, each owning an exclusive
set of fields:

| Controller | Owned fields |
|---|---|
| control-plane-node-controller | `status.caChecksum`, `status.components` |
| control-plane-node-conditions-controller | `status.conditions` |

Rules: fresh read -> deep copy -> modify owned fields only -> diff patch
(`client.MergeFrom` + optimistic lock). Never `Update()`.

The merge patch replaces the whole `status.conditions` list, so the optimistic lock stays: it
guarantees that conditions are never computed from an outdated `status.components` and that
condition types of other writers are never lost. A conflict is returned from `Reconcile`
(ADR controller-standard/050): the next iteration recomputes from a fresh read. Conflicts are
expected when control-plane-node-controller writes `status.components` concurrently.

## Scope and Watches

Node-local (`NODE_NAME` env). `CPN` and `CPO` events are filtered by the label
`control-plane.deckhouse.io/node=<NODE_NAME>`; `Pod` events by namespace `kube-system` and
the mirror pod name of a control plane component on this node (`<component>-<NODE_NAME>`,
e.g. `etcd-<NODE_NAME>`). Labels are not used to identify the pods.

| Resource | Trigger | Mapping |
|---|---|---|
| `ControlPlaneNode` (`For`) | created; spec generation changed; `status.components` changed (except `lastCertObserveTime`) | self |
| owned `ControlPlaneOperation` | created; `spec.approved` flipped; became terminal (`Completed`/`Abandoned`); deleted | owner `ControlPlaneNode` |
| `Pod` (kube-system) | created; deleted; updated with a change of a `PodReady` input (phase, `Ready` pod condition, `CrashLoopBackOff`, deletion timestamp) — control plane mirror pods of this node | the `ControlPlaneNode` of `spec.nodeName` (CPN name = node name) |

Changes of `CPN` `status.conditions` do NOT trigger this controller: they are its own writes
(self-loop). `status.components` changes DO trigger it: control-plane-node-controller
folds operation results into them after the `ControlPlaneOperation` event, so the
operation event alone may run against the old status (`InSync` and `CertificatesHealthy` would stay
outdated). `status.caChecksum` changes do not trigger it: it is not an input of the
conditions (`CertificatesHealthy` compares `spec.caChecksum` with the per-component CA in
`status.components`) and changes only after `status.components` did. `lastCertObserveTime`
is bumped on every certificate observation but is not an input either. Step progress of an operation does not trigger it: `InSync` depends only on the presence
of active operations, not on their steps.
Drift in conditions (e.g. manual edits) is repaired by the periodic requeue (5m).

## Reconciliation Stages

1. Load `CPN`; return on NotFound.
2. List `CPO`s for the node, keep only operations owned by this exact `CPN` UID.
3. Get the control plane mirror pods of this node by name (`<component>-<NODE_NAME>`, the same
   naming as kubelet and control-plane-operation-controller use); a missing pod means "not
   found". Pods are read from the informer cache (the manager client bypasses the cache for
   pods; the Pod watch of this controller keeps the informer running).
4. Compute conditions: `cpnplanner.ComputeConditions(cpn, pods, ops)` (pure).
5. Patch `status.conditions` (optimistic lock) if changed; a conflict is returned as an error.
   Requeue in 5m.

## Condition Logic

### `<Component>PodReady` (pod-based health: `EtcdPodReady`, `APIServerPodReady`,
`ControllerManagerPodReady`, `SchedulerPodReady`)

The component's own static pod on this node is the source of truth — not manifest sync
state, not operation state:

- pod missing -> `False/NotReady` "static pod not found"
- pod terminating (deletion timestamp set) -> `False/NotReady` "static pod `<name>` is terminating"
- pod not `Running` -> `False/NotReady` "static pod phase is `<phase>`"
- pod crash looping -> `False/NotReady` "static pod `<name>` is in CrashLoopBackOff"
- pod `Running` without `Ready` condition -> `False/NotReady` "static pod `<name>` is not ready"
- pod `Running` with `Ready=True` -> `True/Ready`

`ObservedGeneration` is the CPN generation the condition was evaluated at. It does NOT mean
that the pod runs the manifest of that generation.

> **Behavior change.** Previously there was a single `<Component>Ready` per component that
> meant "the manifest converged to the spec" (`True/Ready` when in sync or the operation
> completed, `Unknown/NotReady` when out of sync without an operation, `False/NotReady` while
> an operation waits or runs) and did not look at the pod. Now it is split into
> `<Component>PodReady` (pod health) and `<Component>InSync` (convergence), and
> `<Component>Ready` is their summary. The meaning of `<Component>Ready` is kept and only
> becomes stricter (a broken pod is `False` even when the manifest is in sync), so its
> consumers — dhctl waits for all `<Component>Ready` and `CertificatesHealthy` to be `True`
> (`dhctl/pkg/operations/converge/infrastructure/hook/controlplane/cp-manager.go`) — and the
> `kubectl get cpn` columns need no change.

### `<Component>InSync` (is the desired configuration applied?)

`EtcdInSync`, `APIServerInSync`, `ControllerManagerInSync`, `SchedulerInSync`. An active
operation wins over the checksum comparison; the details live in the operation, `Message`
carries the operation name(s) as a pointer:

- active operation, waiting for approval -> `False/OperationInQueue`, message = name(s)
- active operation approved -> `False/OperationInFlight`, message = name(s)
- no active operation, applied checksums differ from the desired ones -> `False/OutOfSync`,
  message = which checksums differ (`config`, `pki`, `ca`)
- no active operation, checksums match -> `True/InSync`

"Active" is `!IsTerminal()` and not observe-only: terminal is `Completed`/`Abandoned`;
retryable `OperationFailed` operations are still active (they are retried by the operation
controller); observe-only operations (`CertObserve`) do not change the component and are
ignored. The checksum comparison is the planner's `componentState.inSync()`: desired
`spec.components.<c>.checksums.config`/`pki` and `spec.caChecksum` vs applied
`status.components.<c>.checksums` (PKI only for components with PKI).

`OutOfSync` is expected in maintenance mode (no operations are created) and briefly between a
spec change and the operation creation, or between the operation completion and the
`status.components` update. The "one operation per component" invariant is not guaranteed by
construction (converge + signature renewal, defrag hook), so the computation is deterministic
for any number of active operations: `OperationInQueue` wins over `OperationInFlight`, names are ordered by
creation time.

### `<Component>Ready` (summary: `kubectl get cpn` columns, dhctl)

`EtcdReady`, `APIServerReady`, `ControllerManagerReady`, `SchedulerReady` — shown in
the `ETCD`, `APISERVER`, `CONTROLLERMANAGER`, `SCHEDULER` columns; dhctl waits for them
(and `CertificatesHealthy`) before it moves on to the next master. Computed from the two
conditions above in the same reconciliation; reason and message are copied from the failing
one (a not ready pod wins over `InSync`):

- `PodReady` is not `True` -> `False` with `PodReady`'s reason/message (`NotReady`, pod details)
- `InSync` is not `True` -> `False` with `InSync`'s reason/message (`OperationInQueue`/`OperationInFlight`/`OutOfSync`)
- otherwise -> `True/Ready`

### `CertificatesHealthy` (unchanged, from status)

- component without observed cert dates -> `Unknown/Unknown` (`<component>`)
- any component with `status` CA != `spec` CA -> `False/WaitingForComponents` (sorted list)
- any certificate expiring within the renewal threshold (30d) -> `False/ExpiredSoon` (sorted list)
- otherwise -> `True/Healthy`

### Removed components

`PodReady`/`InSync`/`Ready` conditions of components no longer in the spec are pruned on every
reconciliation (`meta.SetStatusCondition` never removes entries). Only the condition types
this controller manages are pruned: condition types of other writers are carried over.