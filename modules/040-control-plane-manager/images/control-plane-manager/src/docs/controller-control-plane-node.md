# control-plane-node

**Name:** `control-plane-node-controller`  
**Primary resource:** `ControlPlaneNode`

## Purpose

Convert `ControlPlaneNode.spec` drift into operations and fold operation results back to `ControlPlaneNode.status`.

## Scope and Watches

This controller is node-local (`NODE_NAME` env) and processes only objects with label:

- `control-plane.deckhouse.io/node=<NODE_NAME>`

| Resource | Trigger | Mapping |
|---|---|---|
| `ControlPlaneNode` | generation changed | self |
| owned `ControlPlaneOperation` | status changed | owner `ControlPlaneNode` |

## Maintenance Mode

When a `ControlPlaneNode` has the `maintenance` label, the controller:
- still updates `CPN.status` from current operations (preserving operation results and component state)
- skips creation of new operations for drift or cert-renewal
- allows manual node maintenance while preserving visibility into operation progress

This mode is useful for manual node maintenance or administrative operations without automatic operation creation interference.

## Code Layout

- `internal/controllers/control-plane-node` - controller wiring (watches, predicates, options).
- `internal/cpn/cpnreconcile` - reconciler: loads state, applies the plan (status patch, create, rotate).
- `internal/cpn/cpnplanner` - pure planning: computes target status and the operations to create/delete.
- `internal/operations` - shared CPO helpers: constructors, dedup predicates, rotation, ownership filter.

## Reconciliation Stages

1. Load `CPN` and all CPOs for this node.
2. Filter operations to objects owned by the current `CPN` UID.
3. Compute the plan (`cpnplanner.ComputePlan`, no API calls):
- target `CPN.status` from operations:
- for each component, choose operation matching current desired checksums (`DesiredConfig/PKI/CA`)
- for component condition, priority is deterministic:
- running (approved, non-terminal) -> pending (not approved) -> completed -> other terminal
- apply checksums from latest terminal non-observe operation that is either:
- `Completed`, or
- has commit-point step completed (`SyncManifests` / `JoinEtcdCluster`)
- apply cert dates from completed operations that include `CertObserve` step, in monotonic `observedAt` order
- update per-component `status.components.<component>.lastCertObserveTime`
- update component conditions and global `CertificatesHealthy`
- in maintenance mode (label `maintenance`) planning stops here: no operations are created or rotated.
- operations to create, per component (lifecycle and renewal are independent decisions):
- lifecycle: converge CPO when `spec != status`; otherwise observe-only CPO when observation is due (interval: 7 days)
- renewal: cert-renewal CPO when certificates expire within threshold (30 days) and the component is in sync; otherwise signature-renewal CPO (CSE only, kube-apiserver)
- a converge CPO already includes cert renewal steps when certificates expire soon
- terminal CPOs to rotate (keep latest 5 per component).
4. Patch `CPN.status` (optimistic lock) if it changed.
5. Create planned CPOs; dedup is re-checked against an uncached list right before creation.
6. Delete rotated CPOs.

Observe-only CPO:
- `spec.component=<real component>`
- `spec.steps=[CertObserve]`
- `spec.approved=true`

## Operation Creation Rules

- Regular drift operations are created only when no active operation with the same desired checksums tuple exists:
- `DesiredConfigChecksum + DesiredPKIChecksum + DesiredCAChecksum`
- Cert-renewal operations are created only when no active operation with `RenewPKICerts` / `RenewKubeconfigs` step exists; signature-renewal - with `RenewSignature` step.
- Observe-only operations are created only when there is no active operation for the component.
- `OperationFailed` is retryable and non-terminal, so a failed CPO with matching desired checksums prevents duplicate CPO creation while it is retried by the CPO controller.
- For regular drift operations, if desired checksums changed while another operation is running, a new operation may be created for the same component.
- Cert-renewal operations are expiry-triggered, but still use the same desired checksums tuple and normal stale/cancel flow in CPO controller.
- CPO name uses `GenerateName` with deterministic prefix:
- `<component>-<short desired checksums>-`
- Steps are selected by component and changed dimensions (`config`, `pki`, `ca`):
- `Backup`, then `SyncCA` + `RenewPKICerts` (etcd, kube-apiserver) + `RenewKubeconfigs` (all except etcd) when certificates change or expire soon, then `RenewSignature` (CSE, kube-apiserver bootstrap/renewal), then the sync step, `WaitPodReady`, `CertObserve`.
- The sync step is `JoinEtcdCluster` for etcd (it also syncs the manifest of an already joined member) and `SyncManifests` for other components.
- After creating a CPO, keep only latest 5 terminal CPOs per component (active CPOs are never deleted).

## Condition Logic (CPN)

- `Synced` when component checksums in status match desired and no operation is needed.
- `PendingUpdate` when matching operation exists but not approved.
- `Updating` when matching operation is approved and running.
- `UpdateFailed` when matching operation is in retryable `OperationFailed` state.
- `CertificatesHealthy=True` when:
- all static pod components report target CA in status, and
- all observed component certificates are not expiring within renewal threshold.

## Logic Basis

- Desired identity of an operation is checksums tuple:
- `DesiredConfigChecksum + DesiredPKIChecksum + DesiredCAChecksum`
- Tuple fields come from `CPN.spec`:
- `DesiredConfigChecksum` <- `spec.components.<component>.checksums.config`
- `DesiredPKIChecksum` <- `spec.components.<component>.checksums.pki`
- `DesiredCAChecksum` <- `spec.caChecksum`
- Checksum composition details are defined in `controller-control-plane-configuration.md` (`Checksum Composition` section).
- Status is derived from operation conditions, not from operation names.
- Commit-point awareness is used when applying results from terminal operations.
- Recreated `CPN` does not restore status from stale history: only operations owned by current `CPN` UID are considered.
