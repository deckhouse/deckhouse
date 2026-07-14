# e2e_log prints a progress message to stderr (visible in chainsaw SCRIPT logs).
e2e_log() {
  printf '[e2e] %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2
}

CPM_E2E_BACKUP_FILE="${TMPDIR:-/tmp}/cpm-e2e-moduleconfig-backup.json"
CPM_E2E_EXISTING_CPOS_FILE="${TMPDIR:-/tmp}/cpm-e2e-existing-cpos.txt"
CPM_E2E_NEW_CPO_FILE="${TMPDIR:-/tmp}/cpm-e2e-new-cpo.txt"
CPM_E2E_AUDIT_FLAG_STATE_FILE="${TMPDIR:-/tmp}/cpm-e2e-audit-flag-state.txt"

CPM_E2E_CP_COMPONENTS="${CPM_E2E_CP_COMPONENTS:-kube-apiserver kube-controller-manager kube-scheduler}"

CPM_E2E_KUBECTL_ATTEMPTS="${CPM_E2E_KUBECTL_ATTEMPTS:-6}"
CPM_E2E_KUBECTL_DELAY="${CPM_E2E_KUBECTL_DELAY:-5}"
CPM_E2E_KUBECTL_REQUEST_TIMEOUT="${CPM_E2E_KUBECTL_REQUEST_TIMEOUT:-30s}"

_kubectl_has_request_timeout() {
  for arg in "$@"; do
    case "$arg" in
      --request-timeout|--request-timeout=*) return 0 ;;
    esac
  done
  return 1
}

_kubectl_not_found_error() {
  case "$1" in
    *"(NotFound)"*) return 0 ;;
  esac
  return 1
}

_kubectl_retryable_error() {
  case "$1" in
    *Conflict*|*"object has been modified"*|\
    *"502 Bad Gateway"*|*"TLS handshake"*|*"connection refused"*|*"Unable to connect"*|*"EOF"*|\
    *"dial tcp"*|*"i/o timeout"*|*"the server is currently unable"*|\
    *"Internal error"*|*"http2: server connection lost"*|*"aggregator"*)
      return 0
      ;;
  esac
  return 1
}

# wait_for_api blocks until the API responds or max_wait seconds elapse.
wait_for_api() {
  max_wait="${1:-60}"
  elapsed=0
  e2e_log "waiting for API (up to ${max_wait}s)"
  while [ "$elapsed" -lt "$max_wait" ]; do
    if kubectl --request-timeout=5s get --raw /healthz >/dev/null 2>&1; then
      e2e_log "API is reachable (/healthz)"
      return 0
    fi
    if kubectl --request-timeout=5s get namespace kube-system >/dev/null 2>&1; then
      e2e_log "API is reachable (namespace/kube-system)"
      return 0
    fi
    e2e_log "API not ready yet (${elapsed}s / ${max_wait}s)"
    sleep 2
    elapsed=$((elapsed + 2))
  done
  e2e_log "API not reachable after ${max_wait}s"
  return 1
}

# kubectl_run waits for the API and retries kubectl on transient or conflict errors.
# NotFound and other permanent errors fail immediately.
# Usage: kubectl_run get pod -n ns
#        output=$(kubectl_run get pod -n ns -o json)
kubectl_run() {
  e2e_log "kubectl $*"
  attempts="$CPM_E2E_KUBECTL_ATTEMPTS"
  delay="$CPM_E2E_KUBECTL_DELAY"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if ! wait_for_api 30; then
      e2e_log "API not ready (attempt $i/$attempts), retrying in ${delay}s"
      sleep "$delay"
      i=$((i + 1))
      continue
    fi
    if _kubectl_has_request_timeout "$@"; then
      kubectl_cmd=(kubectl "$@")
    else
      kubectl_cmd=(kubectl --request-timeout="$CPM_E2E_KUBECTL_REQUEST_TIMEOUT" "$@")
    fi
    if output=$("${kubectl_cmd[@]}" 2>&1); then
      printf '%s' "$output"
      return 0
    fi
    if _kubectl_not_found_error "$output"; then
      e2e_log "kubectl NotFound (no retry): $*"
      echo "$output" >&2
      return 1
    fi
    if _kubectl_retryable_error "$output"; then
      e2e_log "kubectl retryable error (attempt $i/$attempts), retrying in ${delay}s"
      sleep "$delay"
      i=$((i + 1))
      continue
    fi
    e2e_log "kubectl permanent error: $*"
    echo "$output" >&2
    return 1
  done
  e2e_log "kubectl failed after $attempts attempts: $*"
  return 1
}

# wait_until runs a command until it succeeds or timeout_sec elapses.
# Usage: wait_until 300 5 _find_new_component_cpo kube-apiserver "$existing" "$output"
wait_until() {
  timeout_sec="$1"
  interval_sec="$2"
  shift 2
  e2e_log "waiting up to ${timeout_sec}s for: $*"
  deadline=$(( $(date +%s) + timeout_sec ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if "$@"; then
      return 0
    fi
    sleep "$interval_sec"
  done
  e2e_log "timed out after ${timeout_sec}s waiting for: $*"
  return 1
}

# is_flag_in_component returns 0 when needle appears in kube-system pod manifests
# for pods with the given component label.
# Usage: if is_flag_in_component kube-apiserver audit-policy-file; then ...
is_flag_in_component() {
  component="$1"
  needle="$2"
  if [ -z "$component" ] || [ -z "$needle" ]; then
    e2e_log "is_flag_in_component: component and needle are required"
    return 2
  fi
  yaml=$(kubectl_run get pods -n kube-system -l "component=$component" -o yaml)
  if printf '%s' "$yaml" | grep -q "$needle"; then
    e2e_log "'$needle' found in $component pod manifest"
    printf '%s\n' "$yaml" | grep "$needle" >&2 || true
    return 0
  fi
  return 1
}

# backup_moduleconfig_spec saves control-plane-manager ModuleConfig spec for later restore.
# Usage: backup_moduleconfig_spec /path/to/backup.json
# Writes {"notFound":true} when the resource does not exist.
backup_moduleconfig_spec() {
  backup_file="$1"
  if [ -z "$backup_file" ]; then
    e2e_log "backup_moduleconfig_spec: backup file path is required"
    return 1
  fi
  e2e_log "backing up ModuleConfig spec to $backup_file"
  if kubectl_run get moduleconfig control-plane-manager >/dev/null 2>&1; then
    kubectl_run get moduleconfig control-plane-manager -o json | jq '{spec: .spec}' > "$backup_file"
    backed_up=$(jq -r '.spec.settings.apiserver.basicAuditPolicyEnabled // "<unset>"' "$backup_file")
    e2e_log "ModuleConfig spec backed up (basicAuditPolicyEnabled=$backed_up)"
    return 0
  fi
  printf '{"notFound":true}\n' > "$backup_file"
  e2e_log "ModuleConfig not found, backup marked as notFound"
}

# restore_moduleconfig restores ModuleConfig from backup or deletes it when the test created it.
# Usage: restore_moduleconfig /path/to/backup.json
restore_moduleconfig() {
  backup_file="$1"
  if [ -z "$backup_file" ]; then
    e2e_log "restore_moduleconfig: backup file path is required"
    return 1
  fi
  if [ ! -f "$backup_file" ]; then
    e2e_log "no ModuleConfig backup found, skipping restore"
    return 0
  fi
  if jq -e '.notFound == true' "$backup_file" >/dev/null 2>&1; then
    e2e_log "backup had no ModuleConfig, deleting resource created by the test"
    kubectl_run delete moduleconfig control-plane-manager --ignore-not-found
    return 0
  fi

  json_patch=$(jq -c '[{op: "replace", path: "/spec", value: .spec}]' "$backup_file")
  e2e_log "restoring ModuleConfig spec"
  kubectl_run patch moduleconfig control-plane-manager \
    --type=json -p "$json_patch" --request-timeout=60s
  restored=$(kubectl_run get moduleconfig control-plane-manager \
    -o jsonpath='{.spec.settings.apiserver.basicAuditPolicyEnabled}' 2>/dev/null || true)
  e2e_log "ModuleConfig spec restored (basicAuditPolicyEnabled=${restored:-<unset>})"
  e2e_log "waiting for API to stabilize after ModuleConfig restore"
  wait_for_api 120
}

# kubernetes_version returns the cluster Kubernetes minor version (for example 1.34).
kubernetes_version() {
  kubectl_run version -o json | jq -r '[.serverVersion.major, .serverVersion.minor] | join(".")'
}

# snapshot_component_cpos saves existing ControlPlaneOperation names for a component label.
snapshot_component_cpos() {
  component_label="$1"
  cpos_file="$2"
  if [ -z "$component_label" ] || [ -z "$cpos_file" ]; then
    e2e_log "snapshot_component_cpos: component label and file path are required"
    return 1
  fi
  e2e_log "snapshotting $component_label ControlPlaneOperations to $cpos_file"
  kubectl_run get controlplaneoperations -n kube-system \
    -l "control-plane.deckhouse.io/component=$component_label" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' > "$cpos_file" || true
}

# snapshot_control_plane_cpos snapshots ControlPlaneOperations for all control plane components.
snapshot_control_plane_cpos() {
  state_dir="$1"
  if [ -z "$state_dir" ]; then
    e2e_log "snapshot_control_plane_cpos: state directory is required"
    return 1
  fi
  mkdir -p "$state_dir"
  for component_label in $CPM_E2E_CP_COMPONENTS; do
    snapshot_component_cpos "$component_label" "$state_dir/existing-cpos-${component_label}.txt"
  done
}

_find_new_component_cpo() {
  component_label="$1"
  existing_file="$2"
  new_cpo_file="$3"
  names=$(kubectl_run get controlplaneoperations -n kube-system \
    -l "control-plane.deckhouse.io/component=$component_label" \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null) || return 1
  for NAME in $(printf '%s\n' "$names"); do
    [ -z "$NAME" ] && continue
    if ! grep -qxF "$NAME" "$existing_file"; then
      printf '%s' "$NAME" > "$new_cpo_file"
      printf '%s' "$NAME"
      return 0
    fi
  done
  return 1
}

# wait_for_new_component_cpo waits for a ControlPlaneOperation not in the snapshot.
# Prints the new operation name to stdout on success.
wait_for_new_component_cpo() {
  component_label="$1"
  existing_file="$2"
  new_cpo_file="$3"
  timeout="${4:-300}"
  if [ -z "$component_label" ] || [ -z "$existing_file" ] || [ -z "$new_cpo_file" ]; then
    e2e_log "wait_for_new_component_cpo: component label, existing and output file paths are required"
    return 1
  fi
  if wait_until "$timeout" 5 _find_new_component_cpo "$component_label" "$existing_file" "$new_cpo_file"; then
    return 0
  fi
  echo "Timed out waiting for a new $component_label ControlPlaneOperation" >&2
  kubectl_run get controlplaneoperations -n kube-system \
    -l "control-plane.deckhouse.io/component=$component_label" >&2 || true
  return 1
}

# wait_for_new_control_plane_cpos waits for new ControlPlaneOperations on all control plane components.
wait_for_new_control_plane_cpos() {
  state_dir="$1"
  timeout="${2:-300}"
  for component_label in $CPM_E2E_CP_COMPONENTS; do
    wait_for_new_component_cpo "$component_label" \
      "$state_dir/existing-cpos-${component_label}.txt" \
      "$state_dir/new-cpo-${component_label}.txt" \
      "$timeout"
  done
}

# snapshot_flag_state records whether needle appears in component pod manifests (true|false).
# Usage: snapshot_flag_state kube-apiserver audit-policy-file /path/to/state.txt
snapshot_flag_state() {
  component="$1"
  needle="$2"
  state_file="$3"
  if [ -z "$component" ] || [ -z "$needle" ] || [ -z "$state_file" ]; then
    e2e_log "snapshot_flag_state: component, needle, and state file path are required"
    return 1
  fi
  if is_flag_in_component "$component" "$needle"; then
    printf 'true\n' > "$state_file"
    e2e_log "flag snapshot: '$needle' present in $component"
  else
    printf 'false\n' > "$state_file"
    e2e_log "flag snapshot: '$needle' absent in $component"
  fi
}

# assert_flag_state_matches asserts current flag presence matches a snapshot_flag_state file.
# Usage: assert_flag_state_matches kube-apiserver audit-policy-file /path/to/state.txt
assert_flag_state_matches() {
  component="$1"
  needle="$2"
  state_file="$3"
  if [ -z "$component" ] || [ -z "$needle" ] || [ ! -f "$state_file" ]; then
    e2e_log "assert_flag_state_matches: component, needle, and existing state file are required"
    return 1
  fi
  expected=$(tr -d '[:space:]' < "$state_file")
  if [ "$expected" = "true" ]; then
    is_flag_in_component "$component" "$needle"
    return $?
  fi
  if [ "$expected" = "false" ]; then
    if is_flag_in_component "$component" "$needle"; then
      e2e_log "expected '$needle' to remain absent in $component"
      return 1
    fi
    e2e_log "'$needle' remains absent in $component"
    return 0
  fi
  e2e_log "assert_flag_state_matches: invalid snapshot value '$expected' in $state_file"
  return 1
}

# assert_no_new_component_cpo observes for observe_sec and fails if a new CPO appears.
# Usage: assert_no_new_component_cpo kube-apiserver /path/to/existing-cpos.txt 120
assert_no_new_component_cpo() {
  component_label="$1"
  existing_file="$2"
  observe_sec="${3:-120}"
  discard_file="${TMPDIR:-/tmp}/cpm-e2e-discard-cpo.txt"
  if [ -z "$component_label" ] || [ -z "$existing_file" ]; then
    e2e_log "assert_no_new_component_cpo: component label and existing file path are required"
    return 1
  fi
  e2e_log "observing for ${observe_sec}s that no new $component_label ControlPlaneOperation appears"
  deadline=$(( $(date +%s) + observe_sec ))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if _find_new_component_cpo "$component_label" "$existing_file" "$discard_file"; then
      e2e_log "unexpected new $component_label ControlPlaneOperation detected"
      kubectl_run get controlplaneoperations -n kube-system \
        -l "control-plane.deckhouse.io/component=$component_label" -o wide >&2 || true
      return 1
    fi
    sleep 5
  done
  e2e_log "no new $component_label ControlPlaneOperation detected during ${observe_sec}s observation"
  return 0
}

# remove_moduleconfig_maintenance clears spec.maintenance on control-plane-manager ModuleConfig.
remove_moduleconfig_maintenance() {
  e2e_log "removing ModuleConfig spec.maintenance"
  kubectl_run patch moduleconfig control-plane-manager \
    --type=json -p '[{"op":"remove","path":"/spec/maintenance"}]' --request-timeout=60s
}

# apply_or_patch_moduleconfig creates or patches control-plane-manager ModuleConfig from a manifest.
# Usage: apply_or_patch_moduleconfig /path/to/moduleconfig.yaml
apply_or_patch_moduleconfig() {
  target_file="$1"
  if [ -z "$target_file" ]; then
    e2e_log "apply_or_patch_moduleconfig: manifest path is required"
    return 1
  fi
  if kubectl_run get moduleconfig control-plane-manager >/dev/null 2>&1; then
    e2e_log "ModuleConfig exists, patching spec from $target_file"
    patch_payload=$(kubectl create --dry-run=client -o json -f "$target_file" | jq -c '{spec: .spec}')
    kubectl_run patch moduleconfig control-plane-manager \
      --type=merge -p "$patch_payload" --request-timeout=60s
  else
    e2e_log "ModuleConfig not found, creating from $target_file"
    kubectl_run apply -f "$target_file" --request-timeout=60s
  fi
}
