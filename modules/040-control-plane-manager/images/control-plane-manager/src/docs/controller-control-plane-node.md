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

## Reconciliation Stages

1. Load `CPN` and all CPOs for this node.
2. Filter operations to objects owned by the current `CPN` UID.
3. Update `CPN.status` from operations:
- for each component, choose operation matching current desired checksums (`DesiredConfig/PKI/CA`)
- for component condition (`Synced` / `Updating` / `UpdateFailed`), priority is deterministic:
- active (non-terminal) -> completed -> other terminal
- apply checksums from latest terminal operation that is either:
- `Completed`, or
- has commit-point command completed (`SyncManifests` / `JoinEtcdCluster` / `SyncHotReload`)
- apply cert dates from completed operations that include `CertObserve` command, in monotonic `observedAt` order
- update per-component `status.components.<component>.lastObservedAt` (and keep root `status.lastObservedAt` as latest observed timestamp)
- update component conditions, `CASynced`, `CertsRenewal`
4. Check for maintenance mode (label `maintenance`); if present, exit reconciliation (operations remain unchanged).
5. Create missing drift CPOs for components where `spec != status`.
6. Ensure cert-renewal CPO exists for components expiring within threshold (30 days):
- only when component is in-sync (`spec/config,pki,ca == status/config,pki,ca`)
- only when there is no active CPO for this component
- renewal CPO is created with the same `DesiredConfig/PKI/CA` checksums tuple as current component state
7. Ensure periodic observe-only CPO exists per deployed static-pod component (interval: 7 days):
- `spec.component=<real component>`
- `spec.commands=[CertObserve]`
- `spec.approved=true`

## Operation Creation Rules

- Regular drift operations are created only when no active operation with the same desired checksums tuple exists:
- `DesiredConfigChecksum + DesiredPKIChecksum + DesiredCAChecksum`
- Active-operation lookup is unified via shared predicate-based helper (used by regular, renewal, and observe-only creation paths).
- For regular drift operations, if desired checksums changed while another operation is running, a new operation may be created for the same component.
- Cert-renewal operations are expiry-triggered, but still use the same desired checksums tuple and normal stale/cancel flow in CPO controller.
- CPO name uses `GenerateName` with deterministic prefix:
- `<component>-<short desired checksums>-`
- Commands are selected by component and changed dimensions (`config`, `pki`, `ca`).
- Generated command list always starts with `Backup`.
- After creating a CPO, keep only latest 5 terminal CPOs per component (active CPOs are never deleted).

## Condition Logic (CPN)

- `Synced` when component checksums in status match desired and no operation is needed.
- `PendingUpdate` when matching operation exists but not approved.
- `Updating` when matching operation is approved and running.
- `UpdateFailed` when matching operation failed.
- `CASynced=True` only when all static pod components report target CA in status.

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
