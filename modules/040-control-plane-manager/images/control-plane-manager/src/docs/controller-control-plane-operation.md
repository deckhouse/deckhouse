# control-plane-operation

**Name:** `control-plane-operation-controller`  
**Primary resource:** `ControlPlaneOperation`

## Purpose

Execute approved operation pipelines on a specific control-plane node and persist command-level status.

## Scope and Watches

This controller is node-local (`NODE_NAME` env) and watches only CPOs for this node.

| Resource | Trigger | Mapping |
|---|---|---|
| `ControlPlaneOperation` | create/update when operation becomes approved | self |
| `Pod` (kube-system static control-plane pods on this node) | conditions/checksum annotations changed | enqueue approved non-terminal CPOs for same component |

## Reconciliation Logic

1. Load CPO.
2. Skip if not approved or already terminal (`Completed`, `Failed`, `Cancelled`).
3. For non-`CertObserver` operations:
- read `d8-control-plane-manager-config` and `d8-pki`
- verify desired checksums are still current
4. If desired is stale:
- try commit-point recovery for in-progress command
- mark operation `Cancelled` (`Ready=False, Reason=Cancelled`)
5. Execute pipeline commands in declared order.
6. Mark operation succeeded when all commands completed.

## Pipeline and Status Rules

- Every command sets its own condition (`InProgress`, `Completed`, `Failed`).
- `Ready` condition reason reflects active phase (`SyncingManifests`, `WaitingForPod`, etc.).
- Completed commands are skipped on retry/reconcile.
- Failed command is retried on next reconcile.

## Commit Points and Crash Recovery

Commit-point commands:

- `SyncManifests`
- `JoinEtcdCluster`
- `SyncHotReload`

Before cancelling stale desired state, controller can recover an in-progress commit-point if disk/etcd state already matches desired:

- `manifestMatchesDesired` for static pod annotations
- etcd member presence check for `JoinEtcdCluster`
- hot-reload checksum-from-disk check for `SyncHotReload`

## Diff Behavior

- File diff is computed before write in `writeFileIfChanged`.
- Diff files are persisted after command execution via `saveDiffResults`.
- If process crashes after write and before diff save, disk may change without saved diff.

## Logic Basis

- Execution authority: `spec.approved`.
- Desired correctness: checksum matching against current secrets.
- Completion correctness: command conditions + pod readiness/checksum checks.
