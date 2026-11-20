# functions.sh Reference

`functions.sh` lives at the e2e suite root and is symlinked into every scenario's
`scripts/functions.sh` (see `tests/*/scripts/functions.sh`). Every Chainsaw `script`
step sources it the same way:

```sh
. ./scripts/functions.sh
```

`feature-gates.sh` (used only by the `feature-gates` scenario) is sourced *after*
`functions.sh` and builds on top of it — see the note at the end of this file.

All functions are POSIX `sh` (Chainsaw runs scripts with `/usr/bin/sh`, dash on
Ubuntu) — no bash arrays, no `local`.

## Constants

Set at the top of `functions.sh`, overridable via environment:

| Name | Default | Purpose |
| ---- | ------- | ------- |
| `CPM_E2E_BACKUP_FILE` | `${TMPDIR:-/tmp}/cpm-e2e-moduleconfig-backup.json` | `backup_moduleconfig_spec` / `restore_moduleconfig` state |
| `CPM_E2E_EXISTING_CPOS_FILE` | `${TMPDIR:-/tmp}/cpm-e2e-existing-cpos.txt` | CPO snapshot baseline |
| `CPM_E2E_NEW_CPO_FILE` | `${TMPDIR:-/tmp}/cpm-e2e-new-cpo.txt` | Name of the newly detected CPO |
| `CPM_E2E_AUDIT_FLAG_STATE_FILE` | `${TMPDIR:-/tmp}/cpm-e2e-audit-flag-state.txt` | `snapshot_flag_state` / `assert_flag_state_matches` state |
| `CPM_E2E_CP_COMPONENTS` | `kube-apiserver kube-controller-manager kube-scheduler` | Components iterated by the `*_control_plane_cpos` helpers |
| `CPM_E2E_KUBECTL_ATTEMPTS` | `6` | `kubectl_run` retry attempts |
| `CPM_E2E_KUBECTL_DELAY` | `5` | `kubectl_run` delay (seconds) between retries |
| `CPM_E2E_KUBECTL_REQUEST_TIMEOUT` | `30s` | `kubectl_run` default `--request-timeout` |

These file-path constants are what let steps hand state to *later* steps in the
same Chainsaw test — Chainsaw steps don't share shell variables, only the
filesystem (and chainsaw `outputs`), so every "snapshot now, check later" pattern
in these tests goes through one of these files.

## Logging & kubectl core

### `e2e_log`

```sh
e2e_log "message"
```

Prints `[e2e] <UTC timestamp> message` to stderr — shows up as `SCRIPT LOG` /
`SCRIPT ERROR` output in Chainsaw's log. Used throughout every other function
and directly in test scripts for progress/diagnostic messages.

Used in: `tests/basic-audit-policy/chainsaw-test.yaml`, step `assert-operation-complete`

```sh
if [ ! -s "$CPM_E2E_NEW_CPO_FILE" ]; then
  e2e_log "new ControlPlaneOperation file is missing or empty: $CPM_E2E_NEW_CPO_FILE"
  exit 1
fi
```

### `wait_for_api`

```sh
wait_for_api [max_wait_seconds=60]
```

Polls `kubectl get --raw /healthz` (falling back to `kubectl get namespace
kube-system`) every 2s until the API responds or `max_wait_seconds` elapses.
Returns non-zero on timeout. Called internally by `kubectl_run` (30s budget per
attempt) and by `restore_moduleconfig` (120s, after restoring the spec).

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, step `enable-maintenance`

```sh
apply_or_patch_moduleconfig "manifests/moduleconfig-maintenance-only.yaml"
sleep 30
wait_for_api
snapshot_component_cpos kube-apiserver "$CPM_E2E_EXISTING_CPOS_FILE"
```

(Here it's used directly, to make sure the API has stabilized after the
maintenance-enter apply before taking the CPO snapshot the rest of the test
relies on.)

### `kubectl_run`

```sh
kubectl_run <kubectl args...>
output=$(kubectl_run get pod -n ns -o json)
```

The workhorse used for every cluster call in these tests. Before each attempt
it waits (`wait_for_api 30`) for the API to be reachable, then runs `kubectl`
with a `--request-timeout` (defaulted from `CPM_E2E_KUBECTL_REQUEST_TIMEOUT`
unless the caller already passed one). Retries up to `CPM_E2E_KUBECTL_ATTEMPTS`
times with `CPM_E2E_KUBECTL_DELAY`s between attempts on transient errors
(`Conflict`, `502 Bad Gateway`, `TLS handshake`, `connection refused`, `EOF`,
`dial tcp`, `i/o timeout`, aggregator errors, etc. — see `_kubectl_retryable_error`).
`NotFound` errors (`_kubectl_not_found_error`) fail immediately without retry.
Stdout only ever contains `kubectl`'s stdout (safe to pipe into `jq`/`pipe_jq`);
warnings/errors go to stderr.

Used in: `tests/basic-audit-policy/chainsaw-test.yaml`, `catch` block of step `assert-operation-complete`

```sh
kubectl_run get controlplaneoperations -n kube-system \
  -l control-plane.deckhouse.io/component=kube-apiserver -o wide || true
kubectl_run get moduleconfig control-plane-manager -o yaml || true
```

### `pipe_jq`

```sh
kubectl_run version -o json | pipe_jq "kubectl version -o json" -r '.serverVersion.major'
```

Runs `jq "$@"` on stdin. On failure it logs the `source_desc` (first arg) and
dumps the raw input body to stderr before returning non-zero — this is what
makes `jq` parse failures in these tests debuggable instead of a bare "jq:
error" with no context on what was being parsed.

Used in: `tests/feature-gates/scripts/feature-gates.sh`, function `feature_gates_enabled_for_version`

```sh
map_json=$(_feature_gates_map_json "$map_file") || return 1
printf '%s' "$map_json" | pipe_jq "yq -o=json $map_file (feature gates for $version)" --arg ver "$version" '
  .[$ver] as $v
  | if $v == null then error("kubernetes version \($ver) not found in feature gates map") else $v end
  | (($v.deprecated // []) + ($v.forbidden // [])) as $exclude
  | [($v.apiserver // []), ($v.kubeControllerManager // []), ($v.kubeScheduler // []), ($v.kubelet // [])]
  | add | unique
  | map(select(. as $g | ($exclude | index($g) | not)))
'
```

## Generic polling

### `wait_until`

```sh
wait_until <timeout_sec> <interval_sec> <command...>
```

Runs `command...` repeatedly (every `interval_sec`) until it exits `0` or
`timeout_sec` elapses; returns non-zero on timeout. This is the generic
building block behind `wait_for_new_component_cpo` — it isn't called directly
from any `chainsaw-test.yaml`, only from other helpers in this file.

Used internally in: `functions.sh`, `wait_for_new_component_cpo`

```sh
wait_for_new_component_cpo() {
  ...
  if wait_until "$timeout" 5 _find_new_component_cpo "$component_label" "$existing_file" "$new_cpo_file"; then
    return 0
  fi
  ...
}
```

## ModuleConfig backup, apply, and restore

### `backup_moduleconfig_spec`

```sh
backup_moduleconfig_spec <backup_file>
```

Saves `{"spec": ...}` of the current `control-plane-manager` `ModuleConfig` to
`backup_file` (or `{"notFound": true}` if the resource doesn't exist yet).
Always the first thing a scenario does, so `restore_moduleconfig` has
something to put back in `cleanup`.

Used in: `tests/basic-audit-policy/chainsaw-test.yaml`, step `backup-and-snapshot`

```sh
backup_moduleconfig_spec "$CPM_E2E_BACKUP_FILE"
snapshot_component_cpos kube-apiserver "$CPM_E2E_EXISTING_CPOS_FILE"
```

### `restore_moduleconfig`

```sh
restore_moduleconfig <backup_file>
```

Restores the `ModuleConfig` from a `backup_moduleconfig_spec` file: if the
backup says `notFound`, deletes the resource the test created; otherwise
JSON-patches `spec` back with `{op: replace, path: /spec, value: <backed-up spec>}`
(a full replace, not a merge — so settings added during the test that are
absent from the backup are removed). Finishes by waiting up to 120s for the
API to stabilize (`wait_for_api 120`). Always run from a step's `cleanup:`
block so it runs even on failure.

Used in: `tests/basic-audit-policy/chainsaw-test.yaml`, `cleanup` of step `backup-and-snapshot`

```sh
restore_moduleconfig "$CPM_E2E_BACKUP_FILE"
```

### `apply_or_patch_moduleconfig`

```sh
apply_or_patch_moduleconfig <manifest_path>
```

Creates the `control-plane-manager` `ModuleConfig` from `manifest_path` if it
doesn't exist yet, otherwise merge-patches `spec` from the manifest's `spec`
onto the live object (fields not present in the manifest are left untouched —
this is what lets `basic-audit-policy-maintenance` layer a maintenance-only
apply and a settings-only apply without either one clobbering the other).

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, steps `enable-maintenance` and `apply-settings-under-maintenance`

```sh
# step: enable-maintenance — sets only spec.maintenance
apply_or_patch_moduleconfig "manifests/moduleconfig-maintenance-only.yaml"

# step: apply-settings-under-maintenance — merge-patches in the settings change,
# maintenance stays whatever it already was
apply_or_patch_moduleconfig "manifests/moduleconfig-maintenance.yaml"
```

Also used in `tests/basic-audit-policy/chainsaw-test.yaml` (`apply_or_patch_moduleconfig
"manifests/moduleconfig-target.yaml"`) and in `tests/feature-gates/chainsaw-test.yaml`
against the dynamically generated `$CPM_E2E_FG_STATE_DIR/moduleconfig-target.yaml`.

### `remove_moduleconfig_maintenance`

```sh
remove_moduleconfig_maintenance
```

JSON-patches `{op: remove, path: /spec/maintenance}` onto the `ModuleConfig`,
leaving every other setting (e.g. a settings change applied while under
maintenance) in place so it can now reconcile.

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, step `remove-maintenance`

```sh
remove_moduleconfig_maintenance
```

## ControlPlaneOperation (CPO) tracking

### `snapshot_component_cpos`

```sh
snapshot_component_cpos <component_label> <cpos_file>
```

Writes the names of all existing `ControlPlaneOperation`s for
`control-plane.deckhouse.io/component=<component_label>` to `cpos_file`, one
per line. This is the "before" picture that later lets `wait_for_new_component_cpo`
/ `assert_no_new_component_cpo` tell a genuinely new CPO apart from one that
already existed.

Used in: `tests/basic-audit-policy/chainsaw-test.yaml`, step `backup-and-snapshot`

```sh
snapshot_component_cpos kube-apiserver "$CPM_E2E_EXISTING_CPOS_FILE"
```

Also re-run (overwriting the same file) in `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`'s
`enable-maintenance` step, to re-baseline after the maintenance-enter transition
settles — see `apply_or_patch_moduleconfig` above for why that transition is
special-cased.

### `snapshot_control_plane_cpos`

```sh
snapshot_control_plane_cpos <state_dir>
```

Calls `snapshot_component_cpos` for every component in `CPM_E2E_CP_COMPONENTS`
(kube-apiserver, kube-controller-manager, kube-scheduler by default), writing
`<state_dir>/existing-cpos-<component>.txt` for each. The multi-component
counterpart to `snapshot_component_cpos`, used only by `feature-gates` since
it's the only scenario touching all three components at once.

Used in: `tests/feature-gates/chainsaw-test.yaml`, step `backup-and-prepare`

```sh
backup_moduleconfig_spec "$CPM_E2E_BACKUP_FILE"
snapshot_control_plane_cpos "$CPM_E2E_FG_STATE_DIR"
prepare_feature_gates_test "$CPM_E2E_FG_STATE_DIR"
```

### `wait_for_new_component_cpo`

```sh
wait_for_new_component_cpo <component_label> <existing_file> <new_cpo_file> [timeout=300]
```

Polls (every 5s, via `wait_until`) for a `ControlPlaneOperation` on
`component_label` whose name isn't in `existing_file`; writes its name to
`new_cpo_file` (and stdout) as soon as it's found. On timeout, dumps the
current CPOs for that component to stderr for diagnostics.

Used in: `tests/basic-audit-policy/chainsaw-test.yaml`, step `wait-for-operation`

```sh
wait_for_new_component_cpo kube-apiserver "$CPM_E2E_EXISTING_CPOS_FILE" "$CPM_E2E_NEW_CPO_FILE" 300
```

### `wait_for_new_control_plane_cpos`

```sh
wait_for_new_control_plane_cpos <state_dir> [timeout=300]
```

Calls `wait_for_new_component_cpo` for every component in
`CPM_E2E_CP_COMPONENTS`, reading `<state_dir>/existing-cpos-<component>.txt`
(from `snapshot_control_plane_cpos`) and writing
`<state_dir>/new-cpo-<component>.txt` for each.

Used in: `tests/feature-gates/chainsaw-test.yaml`, step `wait-for-operations`

```sh
wait_for_new_control_plane_cpos "$CPM_E2E_FG_STATE_DIR" 300
```

### `assert_no_new_component_cpo`

```sh
assert_no_new_component_cpo <component_label> <existing_file> [observe_sec=120]
```

The inverse of `wait_for_new_component_cpo`: observes for `observe_sec` and
*fails* if any CPO appears on `component_label` that wasn't in `existing_file`.
Used to prove that a change was **not** reconciled (e.g. while
`spec.maintenance: NoResourceReconciliation` is active).

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, step `assert-no-reconciliation`

```sh
assert_no_new_component_cpo kube-apiserver "$CPM_E2E_EXISTING_CPOS_FILE" 60
```

## Flag / manifest state assertions

### `is_flag_in_component`

```sh
if is_flag_in_component <component_label> <needle>; then ...
```

Returns 0 if `needle` (a plain `grep` substring, e.g. a flag name or
`Flag=value` pair) appears anywhere in the YAML of
`kubectl get pods -n kube-system -l component=<component_label> -o yaml`.
Matching lines are also printed to stderr for diagnostics. This is the
lowest-level "does the rendered static pod manifest contain X" check; the
snapshot/assert flag helpers below and `feature-gates.sh`'s
`assert_feature_gates_in_component` are built on top of it.

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, step `assert-no-audit-policy`

```sh
e2e_log "checking kube-apiserver pods for audit-policy-file flag"
if is_flag_in_component kube-apiserver audit-policy-file; then
  exit 1
fi
e2e_log "kube-apiserver pods do not contain audit-policy-file"
```

### `snapshot_flag_state`

```sh
snapshot_flag_state <component_label> <needle> <state_file>
```

Records whether `needle` is currently present (`true`/`false`) in
`component_label`'s pod manifests, to `state_file`. Used to capture a
"before" flag state the same way `snapshot_component_cpos` captures a
"before" CPO list.

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, step `backup-and-snapshot`

```sh
snapshot_flag_state kube-apiserver audit-policy-file "$CPM_E2E_AUDIT_FLAG_STATE_FILE"
```

### `assert_flag_state_matches`

```sh
assert_flag_state_matches <component_label> <needle> <state_file>
```

Asserts the *current* presence of `needle` still matches what
`snapshot_flag_state` recorded in `state_file`. Used to prove a flag's
presence didn't change while it shouldn't have (during maintenance).

Used in: `tests/basic-audit-policy-maintenance/chainsaw-test.yaml`, step `assert-no-reconciliation`

```sh
assert_flag_state_matches kube-apiserver audit-policy-file "$CPM_E2E_AUDIT_FLAG_STATE_FILE"
```

## Misc

### `kubernetes_version`

```sh
version=$(kubernetes_version)   # e.g. "1.34"
```

Returns the cluster's Kubernetes minor version as `<major>.<minor>` by parsing
`kubectl version -o json` through `pipe_jq`.

Used in: `tests/feature-gates/scripts/feature-gates.sh`, function `prepare_feature_gates_test`

```sh
version=$(kubernetes_version)
e2e_log "detected Kubernetes version $version"
enabled_gates=$(feature_gates_enabled_for_version "$version" "$map_file") || return 1
```

## Internal helpers (not part of the public API)

These are prefixed `_` and only ever called from other functions in this
file — don't call them directly from a `chainsaw-test.yaml` step:

| Function | Called by | Purpose |
| -------- | --------- | ------- |
| `_kubectl_has_request_timeout` | `kubectl_run` | Detects whether the caller already passed `--request-timeout` |
| `_kubectl_not_found_error` | `kubectl_run` | Matches `(NotFound)` errors — fail fast, no retry |
| `_kubectl_retryable_error` | `kubectl_run` | Matches transient/conflict errors worth retrying |
| `_find_new_component_cpo` | `wait_for_new_component_cpo`, `assert_no_new_component_cpo` | One-shot "is there a CPO not in the snapshot" check, polled by `wait_until` |

## Building on top: `feature-gates.sh`

The `feature-gates` scenario adds `tests/feature-gates/scripts/feature-gates.sh`,
sourced after `functions.sh`. Its functions (`prepare_feature_gates_test`,
`assert_control_plane_feature_gates`, etc.) call straight into the helpers
above — `kubernetes_version`, `pipe_jq`, `snapshot_control_plane_cpos`,
`wait_for_new_control_plane_cpos`, and `is_flag_in_component` — rather than
reimplementing any cluster interaction. See that file directly for its own
(feature-gates-specific) function reference.
