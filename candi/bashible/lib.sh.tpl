{{- define "bb-status" -}}
function bb-patch-instance-condition() {
  local type="$1"
  local status="$2"
  local reason="$3"
  local message="${4:-}"

  if ! type kubectl >/dev/null 2>&1 || ! test -f /etc/kubernetes/kubelet.conf ; then
    return 0
  fi

  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  local nodeName
  nodeName="$(bb-d8-node-name)"
  bb-kubectl-exec apply --server-side --field-manager=bashible \
    --subresource=status -f - <<EOF || true
apiVersion: deckhouse.io/v1alpha2
kind: Instance
metadata
  name: ${nodeName}
status:
  conditions:
  - type: ${type}
    status: "${status}"
    reason: ${reason}
    message: "${message}"
    lastTransitionTime: "${now}"
EOF
}


function bb-bashible-ready-steps-completed() {
  local last_step="$1"
  local message="${2:-Last successful step: ${last_step}}"
  bb-patch-instance-condition "BashibleReady" "True" "StepsCompleted" "${message}"
}

function bb-bashible-ready-steps-failed() {
  local step="$1"
  local log_excerpt="${2:-}"
  local message="${step}${log_excerpt:+: ${log_excerpt}}"
  bb-patch-instance-condition "BashibleReady" "False" "StepsFailed" "${message}"
}

function bb-bashible-ready-error() {
  local message="${1:-}"
  bb-patch-instance-condition "BashibleReady" "False" "Error" "${message}"
}

function bb-bashible-ready-initial-run() {
  local message="${1:-}"
  bb-patch-instance-condition "BashibleReady" "Unknown" "InitialRunInProgress" "${message}"
}


function bb-waiting-approval-required() {
  local message="${1:-}"
  bb-patch-instance-condition "WaitingApproval" "True" "UpdateApprovalRequired" "${message}"
}

function bb-waiting-approval-not-required() {
  local message="${1:-}"
  bb-patch-instance-condition "WaitingApproval" "False" "UpdateApprovalNotRequired" "${message}"
}


function bb-disruption-approval-required() {
  local message="${1:-}"
  bb-patch-instance-condition "WaitingDisruptionApproval" "True" "DisruptionApprovalRequired" "${message}"
}

function bb-disruption-approval-not-required() {
  local message="${1:-}"
  bb-patch-instance-condition "WaitingDisruptionApproval" "False" "DisruptionApprovalNotRequired" "${message}"
}


function bb-event-create() {
  local event_type="$1" # info или error
  local step="$2"
  local log_note="${3:-}"
  local nodeName
  nodeName="$(bb-d8-node-name)"

  if ! type kubectl >/dev/null 2>&1 || ! test -f /etc/kubernetes/kubelet.conf ; then
    return 0
  fi

  local eventName="bashible-${event_type}-${nodeName}"
  local severity="Normal"
  local reason="BashibleNodeUpdate"

  if [ "$event_type" == "error" ]; then
    severity="Warning"
    reason="BashibleStepFailed"
  fi

  local note
  if [ -n "$log_note" ]; then
    note="${log_note}"
  else
    if [ "$event_type" == "error" ]; then
       note="Bashible step ${step} failed on ${nodeName}"
    else
       note="${step} steps update on ${nodeName}"
    fi
  fi

  local indent="    "
  local indented_note
  indented_note="$(bb-indent-text "$indent" <<<"${note}")"

  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%S.%6NZ")"
  bb-kubectl-exec apply --server-side --field-manager=bashible \
    --namespace=default -f - <<EOF || true
apiVersion: events.k8s.io/v1
kind: Event
metadata
  name: ${eventName}
  namespace: default
  labels:
    bashible.deckhouse.io/node: ${nodeName}
regarding:
  apiVersion: v1
  kind: Node
  name: ${nodeName}
  uid: ${nodeName}
reason: ${reason}
type: ${severity}
note: |
${indented_note}
reportingController: bashible
reportingInstance: ${nodeName}
eventTime: "${now}"
action: "BashibleStepExecution"
EOF
}

function bb-event-error-create() {
  local step="$1"
  local eventLog="/var/lib/bashible/step.log"
  local eventNote

  if [[ -f "${eventLog}" ]]; then
    eventNote="$(tail -c 500 "${eventLog}")"
  else
    eventNote="bashible step log is not available."
  fi

  bb-event-create "error" "${step}" "${eventNote}"
}

function bb-event-info-create() {
  local step="$1"
  bb-event-create "info" "${step}" ""
}
{{- end }}


{{- define "bb-d8-node-name" -}}
bb-d8-node-name() {
  echo $(</var/lib/bashible/discovered-node-name)
}
{{- end }}

{{- define "bb-d8-node-ip" -}}
bb-d8-node-ip() {
  echo $(</var/lib/bashible/discovered-node-ip)
}
{{- end }}

{{- define "bb-discover-node-name" -}}
bb-discover-node-name() {
  local discovered_name_file="/var/lib/bashible/discovered-node-name"
  local kubelet_crt="/var/lib/kubelet/pki/kubelet-server-current.pem"

  if [ ! -s "$discovered_name_file" ]; then
    if [[ -s "$kubelet_crt" ]]; then
      openssl x509 -in "$kubelet_crt" \
        -noout -subject -nameopt multiline |
      awk '/^ *commonName/{print $NF}' | cut -d':' -f3- > "$discovered_name_file"
    else
    {{- if and (ne .nodeGroup.nodeType "Static") (ne .nodeGroup.nodeType "CloudStatic") }}
      if [[ "$(hostname)" != "$(hostname -s)" ]]; then
        hostnamectl set-hostname "$(hostname -s)"
      fi
    {{- end }}
      hostname > "$discovered_name_file"
    fi
  fi
}
{{- end }}

