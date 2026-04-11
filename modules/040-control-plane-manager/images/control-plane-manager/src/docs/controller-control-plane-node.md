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
- apply cert dates from completed operations with `ObservedState` in monotonic `observedAt` order
- update component conditions, `CASynced`, `CertsRenewal`, `LastObservedAt`
4. Create missing CPOs for components where `spec != status`.
5. Ensure periodic `CertObserver` CPO exists (interval: 7 days).
6. Ensure cert-renewal CPO exists for components expiring within threshold (30 days).

## Operation Creation Rules

- Create only when no active operation for the same component exists.
- CPO name uses `GenerateName` with deterministic prefix:
- `<node>-<component>-<short desired checksums>-`
- Commands are selected by component and changed dimensions (`config`, `pki`, `ca`).

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
