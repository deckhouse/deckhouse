#!/usr/bin/env bash

# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
export LANG=C LC_NUMERIC=C
set -Eeo pipefail

{{- $candi := "candi/bashible/lib.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/lib.sh.tpl" -}}
{{- $lib := .Files.Get $deckhouse | default (.Files.Get $candi) -}}
{{- $ctx := . -}}
{{- $clusterMasterKubeAPIEndpoints := list -}}
{{- $clusterMasterRPPAddresses := list -}}
{{- if eq .runType "Normal" -}}
  {{- $clusterMasterEndpoints := .normal.clusterMasterEndpoints | default (list) -}}
  {{- range $endpoint := $clusterMasterEndpoints -}}
    {{- $address := get $endpoint "address" -}}
    {{- if hasKey $endpoint "kubeApiPort" -}}
      {{- $clusterMasterKubeAPIEndpoints = append $clusterMasterKubeAPIEndpoints (printf "%s:%v" $address (get $endpoint "kubeApiPort")) -}}
    {{- end -}}
  {{- end -}}
{{- else -}}
  {{- $clusterMasterEndpoints := .clusterMasterEndpoints | default (list) -}}
  {{- range $endpoint := $clusterMasterEndpoints -}}
    {{- $address := get $endpoint "address" -}}
    {{- if hasKey $endpoint "rppServerPort" -}}
      {{- $clusterMasterRPPAddresses = append $clusterMasterRPPAddresses (printf "%s:%v" $address (get $endpoint "rppServerPort")) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- tpl (printf `
%s

{{ template "bb-d8-node-name" $ }}
{{ template "bb-d8-node-ip" $ }}
{{ template "bb-discover-node-name" $ }}
{{ template "bb-minget" $ }}
` $lib) $ctx }}

bb-label-node-bashible-first-run-finished() {
  local max_attempts=25
  local attempt=1

  while [ $attempt -le $max_attempts ]; do
    if bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "labels" "node.deckhouse.io/bashible-first-run-finished=true"; then
      echo "Successfully set label node.deckhouse.io/bashible-first-run-finished on node $(bb-d8-node-name)"
      return 0
    fi

    echo "[$attempt/$max_attempts] Failed to set label on node $(bb-d8-node-name), retrying in 5 seconds..."
    attempt=$((attempt + 1))
    sleep 5
  done

  echo "ERROR: Timed out after $max_attempts attempts. Could not set label node.deckhouse.io/bashible-first-run-finished on node $(bb-d8-node-name)." >&2
  exit 1
}

bb-node-has-bashible-uninitialized-taint() {
  local max_attempts=5
  local attempt=1

  while [[ $attempt -le $max_attempts ]]; do
    if node_json="$(bb-curl-kube "/api/v1/nodes/$(bb-d8-node-name)" 2>/dev/null)"; then
      if echo "$node_json" | jq -e '.spec.taints[]? | select(.key == "node.deckhouse.io/bashible-uninitialized")' >/dev/null 2>&1; then
        return 0
      else
        return 1
      fi
    fi
    echo "[$attempt/$max_attempts] Failed to get node $(bb-d8-node-name), retrying in 5 seconds..."
    attempt=$((attempt + 1))
    sleep 5
  done

  echo "ERROR: Timed out after $max_attempts attempts. Could not check taint node.deckhouse.io/bashible-uninitialized on node $(bb-d8-node-name)." >&2
  return 1
}

bb-curl-kube-healthz() {
  local server="$1"
  d8-curl -sS -f -x "" --connect-timeout 3 --max-time 3 "${auth_args[@]}" "${server}/healthz" >/dev/null 2>&1
}

bb-curl-kube() {
  local api_path="$1"
  shift

  local kubeconfig="/etc/kubernetes/kubelet.conf"
  local -a auth_args=(--cacert /etc/kubernetes/pki/ca.crt --cert /var/lib/kubelet/pki/kubelet-client-current.pem)

  # If auth type is overridden (admin-cert for cluster-bootstrap), use those creds
  if [[ "${BB_KUBE_AUTH_TYPE:-}" == "admin-cert" ]]; then
    auth_args=(--cacert /etc/kubernetes/pki/ca.crt --cert "${TMPDIR}/bb-kube-admin-cert.pem" --key "${TMPDIR}/bb-kube-admin-key.pem")
    kubeconfig="/etc/kubernetes/admin.conf"
  fi

  if [[ -z "${BB_KUBE_APISERVER_URL:-}" ]]; then
    local kube_server
    kube_server="$(grep -m1 'server:' "$kubeconfig" | awk '{print $2}')"
{{ if eq .runType "Normal" }}
    if [[ -n "$kube_server" ]]; then
      if bb-curl-kube-healthz "$kube_server"; then
        export BB_KUBE_APISERVER_URL="$kube_server"
      else
        for server in {{ $clusterMasterKubeAPIEndpoints | join " " }}; do
          if bb-curl-kube-healthz "https://$server"; then
            export BB_KUBE_APISERVER_URL="https://$server"
            break
          fi
        done
      fi
    fi
{{ end }}
    if [[ -z "${BB_KUBE_APISERVER_URL:-}" ]]; then
      if [[ -n "${kube_server:-}" ]]; then
        export BB_KUBE_APISERVER_URL="$kube_server"
      else
        >&2 echo "bb-curl-kube: cannot resolve API server endpoint"
        return 1
      fi
    fi
  fi

  local rc=0
  d8-curl -sS -f -x "" --connect-timeout 10 --max-time 60 \
    "${auth_args[@]}" \
    "$@" \
    "${BB_KUBE_APISERVER_URL}${api_path}" || rc=$?

  if [[ $rc -ne 0 ]]; then
    BB_KUBE_APISERVER_URL=""
  fi
  return $rc
}

bb-curl-helper-patch-node-metadata() {
  local node_name="$1"
  local field="$2"
  shift 2

  local resource_version=""
  if [[ "${1:-}" == --resource-version=* ]]; then
    resource_version="${1#--resource-version=}"
    shift
  fi

  local json_obj="{}"
  for arg in "$@"; do
    if [[ "$arg" == *=* ]]; then
      local key="${arg%%=*}"
      local value="${arg#*=}"
      json_obj=$(jq --arg k "$key" --arg v "$value" '.[$k] = $v' <<< "$json_obj")
    else
      local key="${arg%-}"
      json_obj=$(jq --arg k "$key" '.[$k] = null' <<< "$json_obj")
    fi
  done

  local patch
  if [[ -n "$resource_version" ]]; then
    patch=$(jq -nc --arg rv "$resource_version" --arg f "$field" --argjson obj "$json_obj" \
      '{"metadata":{"resourceVersion":$rv,($f):$obj}}')
  else
    patch=$(jq -nc --arg f "$field" --argjson obj "$json_obj" \
      '{"metadata":{($f):$obj}}')
  fi

  bb-curl-kube "/api/v1/nodes/${node_name}" \
    -X PATCH \
    -H "Content-Type: application/strategic-merge-patch+json" \
    --data "$patch"
}

# make the function available in $step
export -f bb-curl-kube-healthz
export -f bb-curl-kube
export -f bb-curl-helper-patch-node-metadata
export -f bb-label-node-bashible-first-run-finished
export -f bb-node-has-bashible-uninitialized-taint

bb-indent-text() {
    local indent="$1"
    local line
    while IFS= read -r line || [[ -n "$line" ]]; do
        printf '%s%s\n' "$indent" "$line"
    done
}

function bb-event-error-create() {
    # This function is used for creating event in the default namespace with reference of
    # bashible step and used events.k8s.io/v1 apiVersion.
    # nodeName is used for both .name and .uid fields intentionally as putting a real node uid
    # has proven to have some side effects like missing events when describing objects
    # (https://github.com/deckhouse/deckhouse/issues/4609).
    # If event creation failed, error is suppressed.
    step="$1"
    eventName="$(echo -n "$(bb-d8-node-name)")-$(echo "$step" | sed 's#.*/##; s/_/-/g')"
    nodeName=$(bb-d8-node-name)
    eventLog="/var/lib/bashible/step.log"
    if [[ -f "${eventLog}" ]]; then
      eventNote="$(tail -c 500 "${eventLog}")"
    else
      eventNote="bashible step log is not available."
    fi
    if test -f /etc/kubernetes/kubelet.conf ; then
      local json
      json=$(jq -nc \
        --arg eventName "bashible-error-${eventName}-" \
        --arg nodeName "$nodeName" \
        --arg note "$eventNote" \
        --arg instance "$(bb-d8-node-name)" \
        --arg eventTime "$(date -u +"%Y-%m-%dT%H:%M:%S.%6NZ")" \
        '{
          "apiVersion": "events.k8s.io/v1",
          "kind": "Event",
          "metadata": {"generateName": $eventName},
          "regarding": {"apiVersion": "v1", "kind": "Node", "name": $nodeName, "uid": $nodeName},
          "note": $note,
          "reason": "BashibleStepFailed",
          "type": "Warning",
          "reportingController": "bashible",
          "reportingInstance": $instance,
          "eventTime": $eventTime,
          "action": "BashibleStepExecution"
        }')
      bb-curl-kube "/apis/events.k8s.io/v1/namespaces/default/events" \
        -X POST -H "Content-Type: application/json" --data "$json" >/dev/null 2>&1 || true
    fi
}

function bb-event-info-create() {
    eventName="$(echo -n "$(bb-d8-node-name)")-$(echo "$1" | sed 's#.*/##; s/_/-/g')"
    nodeName="$(bb-d8-node-name)"
    if test -f /etc/kubernetes/kubelet.conf ; then
      local json
      json=$(jq -nc \
        --arg eventName "bashible-info-${eventName}-update-" \
        --arg nodeName "$nodeName" \
        --arg note "$1 steps update on ${nodeName}" \
        --arg instance "$(bb-d8-node-name)" \
        --arg eventTime "$(date -u +"%Y-%m-%dT%H:%M:%S.%6NZ")" \
        '{
          "apiVersion": "events.k8s.io/v1",
          "kind": "Event",
          "metadata": {"generateName": $eventName},
          "regarding": {"apiVersion": "v1", "kind": "Node", "name": $nodeName, "uid": $nodeName},
          "reason": "BashibleNodeUpdate",
          "type": "Normal",
          "note": $note,
          "reportingController": "bashible",
          "reportingInstance": $instance,
          "eventTime": $eventTime,
          "action": "BashibleStepExecution"
        }')
      bb-curl-kube "/apis/events.k8s.io/v1/namespaces/default/events" \
        -X POST -H "Content-Type: application/json" --data "$json" >/dev/null 2>&1 || true
    fi
}

function annotate_node() {
  echo "Annotate node $(bb-d8-node-name) with annotation ${@}"
  attempt=0
  until error=$(bb-curl-helper-patch-node-metadata "$(bb-d8-node-name)" "annotations" "${@}" 2>&1); do
    attempt=$(( attempt + 1 ))
    if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
      >&2 echo "ERROR: Failed to annotate node $(bb-d8-node-name) with annotation ${@} after ${MAX_RETRIES} retries. Last error: ${error}"
      exit 1
    fi
    if [ "$attempt" -gt "2" ]; then
      >&2 echo "Failed to annotate node $(bb-d8-node-name) with annotation ${@} after 3 tries. Last message: ${error}"
      >&2 echo "Retrying..."
      attempt=0
    fi
    sleep 10
  done
  echo "Successful annotate node $(bb-d8-node-name) with annotation ${@}"
}

function get_secret() {
  local secret="$1"

  if test -f /etc/kubernetes/kubelet.conf ; then
    local attempt=0
    until bb-curl-kube "/api/v1/namespaces/d8-cloud-instance-manager/secrets/$secret"; do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Failed to get secret $secret"
        exit 1
      fi
      >&2 echo "failed to get secret $secret"
      sleep 10
    done
{{ if eq .runType "Normal" }}
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    local token="$(</var/lib/bashible/bootstrap-token)"
    while true; do
      for server in {{ $clusterMasterKubeAPIEndpoints | join " " }}; do
        local url="https://$server/api/v1/namespaces/d8-cloud-instance-manager/secrets/$secret"
        if d8-curl -sS -f -x "" --connect-timeout 10 -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
        then
          return 0
        else
          >&2 echo "failed to get secret $secret with curl https://$server..."
        fi
      done
      sleep 10
    done
{{ end }}
  else
    >&2 echo "failed to get secret $secret: can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

function get_bundle() {
  local resource="$1"
  local name="$2"

  if test -f /etc/kubernetes/kubelet.conf ; then
    local attempt=0
    until bb-curl-kube "/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"; do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Failed to get $resource $name"
        exit 1
      fi
      >&2 echo "failed to get $resource $name"
      sleep 10
    done
{{ if eq .runType "Normal" }}
  elif [ -f /var/lib/bashible/bootstrap-token ]; then
    local token="$(</var/lib/bashible/bootstrap-token)"
    while true; do
      for server in {{ $clusterMasterKubeAPIEndpoints | join " " }}; do
        local url="https://$server/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"
        if d8-curl -sS -f -x "" --connect-timeout 10 -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
        then
         return 0
        else
          >&2 echo "failed to get $resource $name with curl https://$server..."
        fi
      done
      sleep 10
    done
{{ end }}
  else
    >&2 echo "failed to get $resource $name: can't find kubelet.conf or bootstrap-token"
    exit 1
  fi
}

log_configuration_checksum() {
  local kind="$1"
  local objName="$2"
  local payload="$3"
  local checksum
  checksum=$(jq -r '.metadata.annotations["bashible.deckhouse.io/configuration-checksum"] // empty' <<<"$payload")
  echo "Got $kind/$objName configuration checksum: $checksum" >&2
}

bootstrap_log_init() {
  local current_file="$1"

  if [[ -z ${BOOTSTRAP_LOG_INITIALIZED:-} ]]; then
    mkdir -p /var/log/d8/bashible
    exec {bootstrap_stdout_fd}>&1
    exec > >(tee -a /var/log/d8/bashible/bootstrap.log >&${bootstrap_stdout_fd}) 2>&1
    export BOOTSTRAP_LOG_INITIALIZED=1
  fi
}

function current_uptime() {
  cat /proc/uptime | cut -d " " -f1
}

function main() {
  export PATH="/opt/deckhouse/bin:/usr/local/bin:$PATH"
  export HOME="/var/lib/bashible"
  export BOOTSTRAP_DIR="/var/lib/bashible"
  export BUNDLE_STEPS_DIR="$BOOTSTRAP_DIR/bundle_steps"
  export CONFIGURATION_CHECKSUM_FILE="$BOOTSTRAP_DIR/configuration_checksum"
  export UPTIME_FILE="$BOOTSTRAP_DIR/uptime"
  export CONFIGURATION_CHECKSUM="{{ .configurationChecksum | default "" }}"
  export FIRST_BASHIBLE_RUN="no"
  export BASHIBLE_INITIALIZED_FILE="$BOOTSTRAP_DIR/bashible-fully-initialized"
  export NODE_GROUP="{{ .nodeGroup.name }}"
  export TMPDIR="/opt/deckhouse/tmp"
  export REGISTRY_MODULE_IGNITER_DIR="$TMPDIR/registry_module_igniter"
  export REGISTRY_MODULE_ENABLE="{{ (.registry).registryModuleEnable | default "false" }}" # Deprecated
  export REGISTRY_MODULE_ADDRESS="registry.d8-system.svc:5001" # Deprecated
  export BB_RP_INSTALLED_PACKAGES_STORE="/var/cache/registrypackages" # Deprecated, backward compatibility

  {{ if eq .runType "Normal" }}
  # autodiscover token and rpp endpoint from kube api
  export PACKAGES_PROXY_KUBE_APISERVER_ENDPOINTS="{{ $clusterMasterKubeAPIEndpoints | join "," }}"
  unset PACKAGES_PROXY_ADDRESSES
  unset PACKAGES_PROXY_TOKEN
  {{ end }}

  {{ if ne .runType "Normal" }}
  # static set of rpp endpoint and token
  export PACKAGES_PROXY_ADDRESSES="{{ $clusterMasterRPPAddresses | join "," }}"
  export PACKAGES_PROXY_TOKEN="{{ get .packagesProxy "token" | default "passthrough" }}"
  {{ end }}

  unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy

  if [ -f "$BOOTSTRAP_DIR/first_run" ] ; then
    FIRST_BASHIBLE_RUN="yes"
    bootstrap_log_init "bashible.sh.tpl"
  fi

  bb-discover-node-name
  export D8_NODE_HOSTNAME=$(bb-d8-node-name)

  if test -f /etc/kubernetes/kubelet.conf ; then
    if tmp="$(bb-curl-kube "/api/v1/nodes/$(bb-d8-node-name)" | jq -r '.metadata.labels."node.deckhouse.io/group"')" ; then
      NODE_GROUP="$tmp"
      if [ "${NODE_GROUP}" == "null" ] ; then
        >&2 echo "failed to get node group. Forgot set label 'node.deckhouse.io/group'"
      fi
    fi
  fi

  mkdir -p "$BUNDLE_STEPS_DIR" "$TMPDIR"

  # update bashible.sh itself
  if [ -z "${BASHIBLE_SKIP_UPDATE-}" ] && [ -z "${is_local-}" ]; then
    bashible_bundle="$(get_bundle bashible "${NODE_GROUP}")"
    log_configuration_checksum "bashible" "${NODE_GROUP}" "$bashible_bundle"
    printf '%s\n' "$bashible_bundle" | jq -r '.data."bashible.sh"' > $BOOTSTRAP_DIR/bashible-new.sh
    if [ ! -s $BOOTSTRAP_DIR/bashible-new.sh ] ; then
      >&2 echo "ERROR: Got empty $BOOTSTRAP_DIR/bashible-new.sh."
      exit 1
    fi
    read -r first_line < $BOOTSTRAP_DIR/bashible-new.sh
    if [[ "$first_line" != '#!/usr/bin/env bash' ]] ; then
      >&2 echo "ERROR: $BOOTSTRAP_DIR/bashible-new.sh is not a bash script."
      exit 1
    fi
    chmod +x $BOOTSTRAP_DIR/bashible-new.sh
    export BASHIBLE_SKIP_UPDATE=yes
    bash --noprofile --norc -c "$BOOTSTRAP_DIR/bashible-new.sh --no-lock"

    # At this step we already know that new version is functional
    mv $BOOTSTRAP_DIR/bashible-new.sh $BOOTSTRAP_DIR/bashible.sh
    sync $BOOTSTRAP_DIR/bashible.sh
    exit 0
  fi

{{ if eq .runType "Normal" }}
  if test -f /etc/kubernetes/kubelet.conf ; then
      REBOOT_ANNOTATION="$( bb-curl-kube "/api/v1/nodes/$D8_NODE_HOSTNAME" |jq -r '.metadata.annotations."update.node.deckhouse.io/reboot"' )"
    else
      REBOOT_ANNOTATION=null
  fi
  if [ "$FIRST_BASHIBLE_RUN" != "yes" ] && [[ ! -f $BASHIBLE_INITIALIZED_FILE ]]; then
     bb-label-node-bashible-first-run-finished
     touch $BASHIBLE_INITIALIZED_FILE
  fi
  if [[ "$FIRST_BASHIBLE_RUN" != "yes" ]] && [[ -f "$BASHIBLE_INITIALIZED_FILE" ]] && test -f /etc/kubernetes/kubelet.conf; then
    if bb-node-has-bashible-uninitialized-taint; then
      echo "WARNING: Node is initialized but bashible-uninitialized taint is still present. Re-applying first-run-finished label..."
      bb-label-node-bashible-first-run-finished
    fi
  fi
  if [[ -f $CONFIGURATION_CHECKSUM_FILE ]] && [[ "$(<$CONFIGURATION_CHECKSUM_FILE)" == "$CONFIGURATION_CHECKSUM" ]] && [[ "$REBOOT_ANNOTATION" == "null" ]] && [[ -f $UPTIME_FILE ]] && [[ "$(<$UPTIME_FILE)" < "$(current_uptime)" ]] 2>/dev/null; then
    echo "Configuration is in sync, nothing to do."
    annotate_node node.deckhouse.io/configuration-checksum=${CONFIGURATION_CHECKSUM}
    current_uptime > $UPTIME_FILE
    exit 0
  fi
  rm -f "$CONFIGURATION_CHECKSUM_FILE"
{{ end }}

  if [ -z "${is_local-}" ]; then
    # update bashbooster library for idempotent scripting
    get_secret bashible-bashbooster | jq -r '.data."bashbooster.sh"' | base64 -d > $BOOTSTRAP_DIR/bashbooster.sh

    # Get steps from bashible apiserver

    rm -rf "$BUNDLE_STEPS_DIR"/*

    nodegroupbundle_bundle="$(get_bundle nodegroupbundle "${NODE_GROUP}")"
    log_configuration_checksum "nodegroupbundle" "${NODE_GROUP}" "$nodegroupbundle_bundle"
    ng_steps_collection="$(printf '%s\n' "$nodegroupbundle_bundle" | jq -rc '.data')"

    for step in $(jq -r 'to_entries[] | .key' <<< "$ng_steps_collection"); do
      jq -r --arg step "$step" '.[$step] // ""' <<< "$ng_steps_collection" > "$BUNDLE_STEPS_DIR/$step"
    done

  fi

  {{- if ne .runType "ClusterBootstrap" }}
      bb-event-info-create "start"
  {{- end }}

  # Execute bashible steps
  for step in $BUNDLE_STEPS_DIR/*; do
    echo ===
    echo === Step: $step
    echo ===
    attempt=0
    sx=""
    until /bin/bash --noprofile --norc -"$sx"eEo pipefail -c "export TERM=xterm-256color; unset CDPATH; cd $BOOTSTRAP_DIR; source /var/lib/bashible/bashbooster.sh; source $step" 2> >(tee /var/lib/bashible/step.log >&2)
    do
      attempt=$(( attempt + 1 ))
      if [ -n "${MAX_RETRIES-}" ] && [ "$attempt" -gt "${MAX_RETRIES}" ]; then
        >&2 echo "ERROR: Failed to execute step $step. Retry limit is over."
        exit 1
      fi
      >&2 echo -e "Failed to execute step "$step" ... retry in 10 seconds.\n"
      sleep 10
      echo ===
      echo === Step: $step
      echo ===
      if [ "$attempt" -gt 2 ]; then
        sx=x
      fi
      {{- if ne .runType "ClusterBootstrap" }}
      bb-event-error-create "$step"
      {{- end }}
    done
  done

  {{- if ne .runType "ClusterBootstrap" }}
      bb-event-info-create "finish"
  {{- end }}

{{ if eq .runType "Normal" }}
  annotate_node node.deckhouse.io/configuration-checksum=${CONFIGURATION_CHECKSUM}

  echo "$CONFIGURATION_CHECKSUM" > $CONFIGURATION_CHECKSUM_FILE
  current_uptime > $UPTIME_FILE
{{ end }}
}

while true ; do
  case ${1:-} in
    --local)
      export is_local=yes
      shift
      ;;
    "--no-lock")
      export no_lock=yes
      shift
      ;;
    "--max-retries")
      export MAX_RETRIES="$2"
      shift
      shift
      ;;
    *)
      break
      ;;
  esac
done

if [ -n "${no_lock-}" ]; then
  main
else
  (
    flock -n 200 || { >&2 echo "Can't acquire lockfile /var/lock/bashible."; exit 1; }
    main
  ) 200>/var/lock/bashible
fi
