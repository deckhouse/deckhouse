{{- define "bootstrap_script" }}
  {{- $context := index . 0 }}
  {{- $ng := index . 1 }}
  {{- $tpl_context := dict }}
  {{- $_ := set $tpl_context "Release" $context.Release }}
  {{- $_ := set $tpl_context "Chart" $context.Chart }}
  {{- $_ := set $tpl_context "Files" $context.Files }}
  {{- $_ := set $tpl_context "Capabilities" $context.Capabilities }}
  {{- $_ := set $tpl_context "Template" $context.Template }}
  {{- $_ := set $tpl_context "Values" $context.Values }}
  {{- $_ := set $tpl_context "nodeGroup" $ng }}
#!/usr/bin/env bash
set -Eeuo pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "${BOOTSTRAP_DIR}" "${TMPDIR}"
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
exec >"${TMPDIR}/bootstrap.log" 2>&1
  {{- end }}

function detect_bundle() {
  {{- $context.Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

function check_python() {
  if command -v python3 >/dev/null 2>&1; then
    python_binary="python3"
    script="$(p3_script)"
    return 0
  fi
  if command -v python2 >/dev/null 2>&1; then
    python_binary="python2"
    script="$(p2_script)"
    return 0
  fi
  if command -v python >/dev/null 2>&1; then
    python_binary="python"
    if python --version | grep -q "Python 3"; then
      script="$(p3_script)"
      return 0
    fi
    if python --version | grep -q "Python 2"; then
      script="$(p2_script)"
      return 0
    fi
  fi
  echo "Python not found, exiting..."
  return 1
}

function p2_script() {
  cat - <<EOF
import os
import sys
import json
import urllib2
import ssl
ssl.match_hostname = lambda cert, hostname: True
request = urllib2.Request(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]})
response = urllib2.urlopen(request, cafile='/var/lib/bashible/ca.crt')
data = json.loads(response.read())
sys.stdout.write(data["bootstrap"])
EOF
}

function p3_script() {
  cat - <<EOF
import sys
import requests
import json
response = requests.get(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]}, verify='/var/lib/bashible/ca.crt')
data = json.loads(response.content)
sys.stdout.write(data["bootstrap"])
EOF
}

function get_phase2() {
  check_python
  bootstrap_bundle_name="${BUNDLE}.{{ $ng.name }}"
  token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  while true; do
    for server in {{ $context.Values.nodeManager.internal.clusterMasterAddresses | join " " }}; do
      url="https://${server}/apis/bashible.deckhouse.io/v1alpha1/bootstrap/${bootstrap_bundle_name}"
      if eval "${python_binary}" - "${url}" "${token}" <<< "${script}"; then
        return 0
      fi
      >&2 echo "failed to get bootstrap ${bootstrap_bundle_name} from $url"
    done
    sleep 10
  done
}

function run_cloud_network_setup() {
  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- if $bootstrap_script_common := $context.Files.Get (printf "candi/cloud-providers/%s/bashible/common-steps/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type)  }}
  cat > ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh <<"EOF"
      {{- tpl $bootstrap_script_common $tpl_context | nindent 0 }}
EOF
  chmod +x ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh
    {{- else }}
      {{- range $path, $_ := $context.Files.Glob (printf "candi/cloud-providers/%s/bashible/bundles/*/bootstrap-networks.sh.tpl" $context.Values.nodeManager.internal.cloudProvider.type) }}
        {{- $bundle := (dir $path | base) }}
  cat > ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks-{{ $bundle }}.sh <<"EOF"
        {{ tpl ($context.Files.Get $path) $tpl_context | nindent 0 }}
EOF
  chmod +x ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks-{{ $bundle }}.sh
      {{- end }}
    {{- end }}
  {{- end }}
  {{- /*
  # Execute cloud provider specific network bootstrap script. It will organize connectivity to kube-apiserver.
  */}}

  if [[ -f ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh ]] ; then
    until ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh; do
      >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
      sleep 10
    done
  elif [[ -f ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks-${BUNDLE}.sh ]] ; then
    until ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks-${BUNDLE}.sh; do
      >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
      sleep 10
    done
  fi
}

  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
function run_log_output() {
  {{- /*
  # Start output bootstrap logs
  */}}
  if type nc >/dev/null 2>&1; then
    tail -n 100 -f ${TMPDIR}/bootstrap.log | nc -l 8000 &
    bootstrap_job_log_pid=$!
  fi
}
 {{- end }}
BUNDLE="$(detect_bundle)"
run_cloud_network_setup
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
run_log_output
  {{- end }}
get_phase2 | bash

  {{- /*
# Stop output bootstrap logs
  */}}
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
if [ -n "${bootstrap_job_log_pid}" ]; then
  kill -9 "${bootstrap_job_log_pid}"
fi
  {{- end }}
{{- end }}
