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
	{{- $_ := set $tpl_context "images" $context.Values.global.modulesImages.digests.registrypackages }}
#!/usr/bin/env bash
set -Eeuo pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "${BOOTSTRAP_DIR}" "${TMPDIR}"
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
exec >"${TMPDIR}/bootstrap.log" 2>&1
  {{- end }}

  {{- if $fetch_base_pkgs := $context.Files.Get "candi/bashible/bootstrap/01-base-pkgs.sh.tpl" }}
function prepare_base_d8_binaries() {
    {{- tpl $fetch_base_pkgs $tpl_context | nindent 2 }}
}
  {{- end }}

  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- if $bootstrap_networks := $context.Files.Get "candi/bashible/bootstrap/02-network-scripts.sh.tpl" }}
function run_cloud_network_setup() {
      {{- tpl $bootstrap_networks $tpl_context | nindent 2 }}
    {{- end }}
return 0
}
  {{- end }}
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
function run_log_output() {
  if type nc >/dev/null 2>&1; then
    tail -n 100 -f ${TMPDIR}/bootstrap.log | nc -l -p 8000 &
    bootstrap_job_log_pid=$!
  fi
}
  {{- end }}

function load_phase2_script() {
  cat - <<EOF
import sys
import json
import ssl

try:
    from urllib.request import urlopen, Request
except ImportError as e:
    from urllib2 import urlopen, Request

ssl.match_hostname = lambda cert, hostname: True
request = Request(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]})
response = urlopen(request, cafile='/var/lib/bashible/ca.crt')
data = json.loads(response.read())
sys.stdout.write(data["bootstrap"])
EOF
}


function get_phase2() {
  bootstrap_ng_name="common.{{ $ng.name }}"
  token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  while true; do
    for server in {{ $context.Values.nodeManager.internal.clusterMasterAddresses | join " " }}; do
      url="https://${server}/apis/bashible.deckhouse.io/v1alpha1/bootstrap/${bootstrap_ng_name}"
      if eval "${python_binary}" - "${url}" "${token}" <<< "$(load_phase2_script)"; then
        return 0
      fi
      >&2 echo "failed to get bootstrap ${bootstrap_ng_name} from $url"
    done
    sleep 10
  done
}

prepare_base_d8_binaries
  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
run_cloud_network_setup
  {{- end }}
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
