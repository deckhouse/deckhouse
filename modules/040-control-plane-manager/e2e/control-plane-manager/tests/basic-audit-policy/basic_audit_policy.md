# Apiserver Operation

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates the **control-plane-manager** module creates and completes a kube-apiserver `ControlPlaneOperation` when `apiserver.basicAuditPolicyEnabled` is changed.

**What it does:** Backs up the current `ModuleConfig`, snapshots existing kube-apiserver `ControlPlaneOperation` resources, applies the target module settings (`basicAuditPolicyEnabled: false`), waits for a newly created operation (ignoring pre-existing ones), and asserts the operation pipeline and completion. Restores the original `ModuleConfig` on completion or failure.

## Prerequisites

- `control-plane-manager` module installed and running in the cluster
- At least one control-plane node with a running kube-apiserver static pod
- Chainsaw CLI and `kubectl` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
| ---- | ---- | ----------- |
| 1 | `backup-and-snapshot` | Backs up `ModuleConfig` spec and snapshots existing kube-apiserver `ControlPlaneOperation` names |
| 2 | `apply-moduleconfig` | Patches or creates `ModuleConfig` with target settings (`basicAuditPolicyEnabled: false`) |
| 3 | `wait-for-operation` | Waits for a newly created kube-apiserver `ControlPlaneOperation` |
| 4 | `assert-operation-complete` | Asserts operation steps and `Completed`/`OperationCompleted` condition |
| 5 | `assert-no-audit-policy` | Asserts kube-apiserver pods do not contain `audit-policy-file` |

**Cleanup:** Step 1 (`backup-and-snapshot`) cleanup restores the original `ModuleConfig` at test end (Chainsaw runs step cleanups in reverse order, so step 1 runs last). Restore replaces the entire `spec` via JSON patch (not merge), so fields added during the test (e.g. `basicAuditPolicyEnabled: false`) are removed when absent from the backup. Backup stores only `spec` — no `resourceVersion`.

## Files

| File | Purpose |
| ---- | ------- |
| `chainsaw-test.yaml` | Chainsaw test definition |
| `manifests/moduleconfig-target.yaml` | Target `ModuleConfig` (`basicAuditPolicyEnabled: false`) |
| `scripts/kubectl-retry.sh` | Shared `kubectl_retry` helper (retries transient API errors for up to ~1 minute) |

## API availability

Applying `ModuleConfig` restarts kube-apiserver, so the Kubernetes API may be briefly unavailable. `scripts/functions.sh` provides helpers:

- `e2e_log` — progress messages with UTC timestamp
- `kubectl_run` — waits for the API and retries kubectl on transient or conflict errors; `NotFound` fails immediately
- `wait_until <timeout> <interval> <command...>` — polls a command until it succeeds or times out
- `apply_or_patch_moduleconfig <path>` — creates or merge-patches `control-plane-manager` ModuleConfig from a manifest
- `backup_moduleconfig_spec <path>` / `restore_moduleconfig <path>` — backup and restore `control-plane-manager` ModuleConfig spec
- `snapshot_component_cpos <label> <path>` — snapshots existing ControlPlaneOperations for a component
- `wait_for_new_component_cpo <label> <existing> <output> [timeout]` — waits for a new ControlPlaneOperation
- `is_flag_in_component <component> <needle>` — returns 0 when needle appears in kube-system pod manifests for the component label

Test scripts call `kubectl_run` for cluster operations; retries, API waits, and polling loops live in `functions.sh`.

Completion and steps are verified with a Chainsaw `assert` (10m timeout), which polls the resource until it matches or times out.

The test uses `namespace: default` (a pre-existing namespace) so Chainsaw does not create or delete an ephemeral test namespace after cleanup.

## ModuleConfig Target Settings

The test applies:

```yaml
spec:
  enabled: true
  settings:
    apiserver:
      basicAuditPolicyEnabled: false
  version: 3
```

If the `ModuleConfig` exists, the test patches `spec` via merge patch. Otherwise it creates the resource from `moduleconfig-target.yaml`.

## ControlPlaneOperation Detection

Existing kube-apiserver operations are captured before the `ModuleConfig` change. The test waits for a `ControlPlaneOperation` in `kube-system` with label `control-plane.deckhouse.io/component: kube-apiserver` whose name was not present in the initial snapshot.

## Running

```bash
# From the test directory
task run

# From control-plane-manager e2e root
task basic-audit-policy:run

# Or directly
chainsaw test --test-dir . --config ../../chainsaw-config.yaml
```

## Pass/Fail Criteria

- **Pass:** A new kube-apiserver `ControlPlaneOperation` appears with `spec.steps` `[Backup, SyncManifests, WaitPodReady, CertObserve]` and a `Completed` condition with `reason: OperationCompleted` and `status: "True"`; kube-apiserver pods in `kube-system` do not contain the `audit-policy-file` flag
- **Fail:** No new operation within 5 minutes, unexpected steps, operation does not complete within the assert timeout (10 minutes), or `audit-policy-file` is present in kube-apiserver pod manifests

## Troubleshooting

### No new ControlPlaneOperation

Check module status and recent operations:

```bash
kubectl get moduleconfig control-plane-manager -o yaml
kubectl get controlplaneoperations -n kube-system -l control-plane.deckhouse.io/component=kube-apiserver
kubectl logs -n kube-system -l app=d8-control-plane-manager --tail=100
```

### Operation stuck or failed

Inspect the operation status and kube-apiserver static pod on the target node:

```bash
kubectl get controlplaneoperations -n kube-system -l control-plane.deckhouse.io/component=kube-apiserver -o yaml
kubectl get pods -n kube-system -l component=kube-apiserver
```

### ModuleConfig not restored

If the test process was killed before cleanup, restore manually from the backup file:

```bash
BACKUP_FILE="${TMPDIR:-/tmp}/cpm-e2e-moduleconfig-backup.json"
kubectl patch moduleconfig control-plane-manager --type=json \
  -p "$(jq -c '[{op: "replace", path: "/spec", value: .spec}]' "$BACKUP_FILE")"
Or source the helper: `. ./scripts/functions.sh && restore_moduleconfig "$BACKUP_FILE"`
P_FILE"`

## Safety

This test modifies the cluster `control-plane-manager` `ModuleConfig`, which triggers a real kube-apiserver reconciliation on a control-plane node. The cleanup block restores the original configuration. Plan for a short apiserver static pod restart during the test.
