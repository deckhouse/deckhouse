# Basic Audit Policy — Maintenance Mode

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **control-plane-manager** does not reconcile `ModuleConfig` settings while `spec.maintenance` is `NoResourceReconciliation`, then reconciles normally once maintenance is cleared.

**What it does:** Backs up the current `ModuleConfig`, snapshots existing kube-apiserver `ControlPlaneOperation` resources and the initial `audit-policy-file` flag state, then enables `spec.maintenance: NoResourceReconciliation` **on its own** (no settings change yet). Entering maintenance forces exactly one Helm-release upgrade at the `deckhouse-controller` level to stamp the maintenance marker on the release resources — this is unrelated to any settings change and must be allowed to settle before the real check. Once settled, the test re-snapshots CPOs and only then applies `basicAuditPolicyEnabled: false` while already under maintenance, asserts no new operation appears and apiserver flags are unchanged, removes `spec.maintenance`, waits for reconciliation (same as `basic-audit-policy`), and asserts the operation completes with audit policy removed. Restores the original `ModuleConfig` on completion or failure.

**Why the two-step apply matters:** applying `maintenance` and a settings change in the *same* patch does not test what it looks like it tests. `NoResourceReconciliation` is enforced by `deckhouse-controller`'s Helm-release engine, not by control-plane-manager itself, and that engine always lets exactly one upgrade through on the enter/leave transition. If the settings change rides along in the same apply as entering maintenance, it *is* that one forced upgrade and goes through immediately — a real kube-apiserver `ControlPlaneOperation` gets created right away, and the test would be asserting the wrong thing. Splitting the apply into "enter maintenance" then "change settings while already under maintenance" is what actually exercises the block.

## Prerequisites

- `control-plane-manager` module installed and running in the cluster
- At least one control-plane node with a running kube-apiserver static pod
- Chainsaw CLI and `kubectl` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
| ---- | ---- | ----------- |
| 1 | `backup-and-snapshot` | Backs up `ModuleConfig` spec, snapshots existing kube-apiserver `ControlPlaneOperation` names, and records initial `audit-policy-file` flag state |
| 2 | `enable-maintenance` | Patches or creates `ModuleConfig` with `maintenance: NoResourceReconciliation` alone, waits for the forced enter-transition upgrade to settle, then re-snapshots kube-apiserver `ControlPlaneOperation` names |
| 3 | `apply-settings-under-maintenance` | Patches `ModuleConfig` with `basicAuditPolicyEnabled: false` while maintenance is already active |
| 4 | `assert-no-reconciliation` | Observes for 60s that no new kube-apiserver `ControlPlaneOperation` appears and `audit-policy-file` flag state is unchanged |
| 5 | `remove-maintenance` | Removes `spec.maintenance` via JSON patch while keeping target settings |
| 6 | `wait-for-operation` | Waits for a newly created kube-apiserver `ControlPlaneOperation` |
| 7 | `assert-operation-complete` | Asserts operation steps and `Completed`/`OperationCompleted` condition |
| 8 | `assert-no-audit-policy` | Asserts kube-apiserver pods do not contain `audit-policy-file` |

**Cleanup:** Step 1 (`backup-and-snapshot`) cleanup restores the original `ModuleConfig` at test end.

## Files

| File | Purpose |
| ---- | ------- |
| `chainsaw-test.yaml` | Chainsaw test definition |
| `manifests/moduleconfig-maintenance-only.yaml` | `ModuleConfig` with only `maintenance: NoResourceReconciliation` set (step 2) |
| `manifests/moduleconfig-maintenance.yaml` | `ModuleConfig` with maintenance mode and `basicAuditPolicyEnabled: false` (step 3, and again after maintenance is removed) |
| `scripts/functions.sh` | Symlink to `../../../functions.sh` |

## ModuleConfig Target Settings

Step 2 (`enable-maintenance`) applies maintenance alone:

```yaml
spec:
  maintenance: NoResourceReconciliation
  enabled: true
  version: 3
```

Step 3 (`apply-settings-under-maintenance`) then applies the settings change, with maintenance already active and unchanged:

```yaml
spec:
  maintenance: NoResourceReconciliation
  enabled: true
  settings:
    apiserver:
      basicAuditPolicyEnabled: false
  version: 3
```

## Running

```bash
# From the test directory
task run

# From control-plane-manager e2e root
task basic-audit-policy-maintenance:run

# Or directly
chainsaw test --test-dir . --config ../../chainsaw-config.yaml
```

## Pass/Fail Criteria

- **Pass:** No new kube-apiserver `ControlPlaneOperation` during maintenance; `audit-policy-file` flag unchanged during maintenance; after removing maintenance, a new operation completes with expected steps and kube-apiserver pods no longer contain `audit-policy-file`
- **Fail:** New operation appears during maintenance, audit flag changes during maintenance, no operation after maintenance removal, operation does not complete, or `audit-policy-file` remains after reconciliation

## Safety

This test modifies the cluster `control-plane-manager` `ModuleConfig`. During the reconciliation phase (after maintenance removal), expect a brief kube-apiserver static pod restart. Cleanup restores the backed-up configuration.
