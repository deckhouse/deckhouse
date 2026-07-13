# e2e_log prints a progress message to stderr (visible in chainsaw SCRIPT logs).
e2e_log() {
  printf '[e2e] %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" >&2
}

# kubectl_run logs and executes a kubectl command.
# Usage: kubectl_run get pod -n ns
#        output=$(kubectl_run get pod -n ns -o json 2>&1)
kubectl_run() {
  e2e_log "kubectl $*"
  kubectl "$@"
}

CPM_E2E_BACKUP_FILE="${TMPDIR:-/tmp}/cpm-e2e-moduleconfig-backup.json"
CPM_E2E_EXISTING_CPOS_FILE="${TMPDIR:-/tmp}/cpm-e2e-existing-cpos.txt"
CPM_E2E_NEW_CPO_FILE="${TMPDIR:-/tmp}/cpm-e2e-new-cpo.txt"

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

# kubectl_mutate waits for API once, then runs a single kubectl command.
# Use for apply/patch/delete when the API was recently verified.
kubectl_mutate() {
  wait_for_api 30 || return 1
  kubectl_run --request-timeout=60s "$@"
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

# kubectl_retry waits for API, then runs kubectl with retries on transient errors.
# NotFound and other permanent errors fail immediately without retrying.
# Usage: output=$(kubectl_retry get pod foo -n bar)
kubectl_retry() {
  attempts=6
  delay=5
  i=1
  while [ "$i" -le "$attempts" ]; do
    wait_for_api 30 || return 1
    if output=$(kubectl --request-timeout=30s "$@" 2>&1); then
      printf '%s' "$output"
      return 0
    fi
    case $output in
      *"(NotFound)"*)
        e2e_log "kubectl NotFound (no retry): $*"
        echo "$output" >&2
        return 1
        ;;
      *"502 Bad Gateway"*|*"TLS handshake"*|*"connection refused"*|*"Unable to connect"*|*"EOF"*|\
      *"dial tcp"*|*"i/o timeout"*|*"the server is currently unable"*|\
      *"Internal error"*|*"http2: server connection lost"*|*"aggregator"*)
        e2e_log "kubectl transient error (attempt $i/$attempts), retrying in ${delay}s"
        sleep "$delay"
        i=$((i + 1))
        ;;
      *)
        e2e_log "kubectl permanent error: $*"
        echo "$output" >&2
        return 1
        ;;
    esac
  done
  e2e_log "kubectl failed after $attempts attempts: $*"
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
    kubectl_retry delete moduleconfig control-plane-manager --ignore-not-found
    return 0
  fi

  json_patch=$(jq -c '[{op: "replace", path: "/spec", value: .spec}]' "$backup_file")
  attempts=6
  delay=5
  i=1
  while [ "$i" -le "$attempts" ]; do
    wait_for_api 30 || return 1
    e2e_log "restoring ModuleConfig spec (attempt $i/$attempts)"
    if output=$(kubectl --request-timeout=30s patch moduleconfig control-plane-manager \
      --type=json -p "$json_patch" 2>&1); then
      restored=$(kubectl_run get moduleconfig control-plane-manager \
        -o jsonpath='{.spec.settings.apiserver.basicAuditPolicyEnabled}' 2>/dev/null || true)
      e2e_log "ModuleConfig spec restored (basicAuditPolicyEnabled=${restored:-<unset>})"
      return 0
    fi
    case $output in
        *Conflict*|*"object has been modified"*)
          e2e_log "ModuleConfig conflict during restore, retrying in ${delay}s"
          ;;
        *"502 Bad Gateway"*|*"TLS handshake"*|*"connection refused"*|*"Unable to connect"*|*"EOF"*|\
        *"dial tcp"*|*"i/o timeout"*|*"the server is currently unable"*|\
        *"Internal error"*|*"http2: server connection lost"*|*"aggregator"*)
          e2e_log "kubectl transient error during restore (attempt $i/$attempts), retrying in ${delay}s"
          ;;
        *)
          e2e_log "ModuleConfig restore failed"
          echo "$output" >&2
          return 1
          ;;
    esac
    sleep "$delay"
    i=$((i + 1))
  done
  e2e_log "ModuleConfig restore failed after $attempts attempts"
  return 1
}



# snapshot_kube_apiserver_cpos saves existing kube-apiserver ControlPlaneOperation names.
# Usage: snapshot_kube_apiserver_cpos /path/to/file
snapshot_kube_apiserver_cpos() {
  cpos_file="$1"
  if [ -z "$cpos_file" ]; then
    e2e_log "snapshot_kube_apiserver_cpos: file path is required"
    return 1
  fi
  e2e_log "snapshotting kube-apiserver ControlPlaneOperations to $cpos_file"
  kubectl_run get controlplaneoperations -n kube-system \
    -l control-plane.deckhouse.io/component=kube-apiserver \
    -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' > "$cpos_file" || true
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
    kubectl_mutate patch moduleconfig control-plane-manager --type=merge -p "$patch_payload"
  else
    e2e_log "ModuleConfig not found, creating from $target_file"
    kubectl_mutate apply -f "$target_file"
  fi
}
