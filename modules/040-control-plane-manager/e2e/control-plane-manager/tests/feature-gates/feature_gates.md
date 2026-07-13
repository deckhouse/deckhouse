# Feature Gates

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test that validates **control-plane-manager** applies supported feature gates for the cluster Kubernetes version to kube-apiserver, kube-controller-manager, and kube-scheduler.

**What it does:** Detects the cluster minor Kubernetes version, reads `candi/feature_gates_map.yml`, builds `enabledFeatureGates` from all component lists (unique, excluding `forbidden` and `deprecated`), applies the `ModuleConfig`, waits for new `ControlPlaneOperation` resources on all three control plane components, asserts completion, and verifies each component manifest contains `GateName=true` for its gates.

## Prerequisites

- `control-plane-manager` module installed and running in the cluster
- Control-plane nodes with kube-apiserver, kube-controller-manager, and kube-scheduler static pods
- Chainsaw CLI, `kubectl`, `jq`, and `yq` installed. See `../../README.md` for instructions.

## Test Steps

| Step | Name | Description |
| ---- | ---- | ----------- |
| 1 | `backup-and-prepare` | Backs up `ModuleConfig`, snapshots existing CPOs, builds target manifest from `feature_gates_map.yml` |
| 2 | `apply-moduleconfig` | Patches or creates `ModuleConfig` with dynamic `enabledFeatureGates` |
| 3 | `wait-for-operations` | Waits for new CPOs on kube-apiserver, kube-controller-manager, kube-scheduler |
| 4 | `assert-operations-complete` | Chainsaw asserts on each new CPO for steps and `Completed`/`OperationCompleted` |
| 5 | `assert-feature-gates` | Asserts each component pod manifest contains its feature gates as `GateName=true` |

**Cleanup:** Step 1 cleanup restores the original `ModuleConfig` at test end.

## Files

| File | Purpose |
| ---- | ------- |
| `chainsaw-test.yaml` | Chainsaw test definition |
| `scripts/functions.sh` | Symlink to shared kubectl/CPO helpers |
| `scripts/feature-gates.sh` | Feature gates map parsing and ModuleConfig generation |
| `manifests/moduleconfig-target.yaml` | Example only; the test generates the real manifest at runtime |

## Dynamic ModuleConfig

At runtime `prepare_feature_gates_test`:

1. Reads Kubernetes minor version from the API server (`kubectl version`, e.g. `1.34`)
2. Loads `../../../../../candi/feature_gates_map.yml` (override with `CPM_E2E_FEATURE_GATES_MAP`)
3. Unions gates from `apiserver`, `kubeControllerManager`, `kubeScheduler`, and `kubelet`, excluding `forbidden` and `deprecated`
4. Writes `${CPM_E2E_FG_STATE_DIR}/moduleconfig-target.yaml`

State files live in `${TMPDIR:-/tmp}/cpm-e2e-feature-gates/` by default.

## Running

```bash
# From the test directory
task run

# From control-plane-manager e2e root
task feature-gates:run
```

## Pass/Fail Criteria

- **Pass:** New CPOs appear for all three components, complete with expected steps, and each component manifest contains its feature gates
- **Fail:** Version missing from map, no new CPO within timeout, operation incomplete, or a feature gate missing from a component manifest

## Safety

This test modifies the cluster `control-plane-manager` `ModuleConfig` with all supported feature gates for the current Kubernetes version, triggering control plane reconciliations. Cleanup restores the original configuration.
