# Basic Audit Policy — Maintenance Mode

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **control-plane-manager** does not reconcile `ModuleConfig` settings while `spec.maintenance` is `NoResourceReconciliation`, then reconciles normally once maintenance is cleared.

**What it does:** Backs up the current `ModuleConfig`, snapshots existing kube-apiserver `ControlPlaneOperation` resources and the initial `audit-policy-file` flag state, applies the target settings with maintenance mode enabled, asserts no new operation appears and apiserver flags are unchanged, removes `spec.maintenance`, waits for reconciliation (same as `basic-audit-policy`), and asserts the operation completes with audit policy removed. Restores the original `ModuleConfig` on completion or failure.

## Prerequisites

- `control-plane-manager` module installed and running in the cluster
- At least one control-plane node with a running kube-apiserver static pod
- Chainsaw CLI and `kubectl` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
| ---- | ---- | ----------- |
| 1 | `backup-and-snapshot` | Backs up `ModuleConfig` spec, snapshots existing kube-apiserver `ControlPlaneOperation` names, and records initial `audit-policy-file` flag state |
| 2 | `apply-moduleconfig-maintenance` | Patches or creates `ModuleConfig` with `maintenance: NoResourceReconciliation` and `basicAuditPolicyEnabled: false` |
| 3 | `assert-no-reconciliation` | Observes for 120s that no new kube-apiserver `ControlPlaneOperation` appears and `audit-policy-file` flag state is unchanged |
| 4 | `remove-maintenance` | Removes `spec.maintenance` via JSON patch while keeping target settings |
| 5 | `wait-for-operation` | Waits for a newly created kube-apiserver `ControlPlaneOperation` |
| 6 | `assert-operation-complete` | Asserts operation steps and `Completed`/`OperationCompleted` condition |
| 7 | `assert-no-audit-policy` | Asserts kube-apiserver pods do not contain `audit-policy-file` |

**Cleanup:** Step 1 (`backup-and-snapshot`) cleanup restores the original `ModuleConfig` at test end.

## Files

| File | Purpose |
| ---- | ------- |
| `chainsaw-test.yaml` | Chainsaw test definition |
| `manifests/moduleconfig-maintenance.yaml` | Target `ModuleConfig` with maintenance mode and `basicAuditPolicyEnabled: false` |
| `scripts/functions.sh` | Symlink to `../../../functions.sh` |

## ModuleConfig Target Settings (maintenance phase)

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
