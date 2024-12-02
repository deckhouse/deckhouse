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
  {{- $_ := set $tpl_context "images" $context.Values.global.modulesImages.digests }}
  {{- $_ := set $tpl_context "packagesProxy" $context.Values.nodeManager.internal.packagesProxy }}
  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
    {{- $_ := set $tpl_context "provider" $context.Values.nodeManager.internal.cloudProvider.type }}
  {{- end }}
#!/usr/bin/env bash
set -Eeuo pipefail

BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "${BOOTSTRAP_DIR}" "${TMPDIR}"
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
exec >"${TMPDIR}/bootstrap.log" 2>&1
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
  bootstrap_ng_name="{{ $ng.name }}"
  token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  check_python
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

  {{- if $bb_package_install := $context.Files.Get "candi/bashible/bb_package_install.sh.tpl" -}}
    {{- tpl ( $bb_package_install ) $tpl_context | nindent 0 }}
  {{- end }}

#run network scripts
  {{- if $bootstrap_networks := $context.Files.Get "candi/bashible/bootstrap/01-network-scripts.sh.tpl" }}
    {{- tpl ( $bootstrap_networks ) $tpl_context | nindent 0 }}
  {{- end }}

#prepare_base_d8_binaries
  {{- if $fetch_base_pkgs := $context.Files.Get "candi/bashible/bootstrap/02-base-pkgs.sh.tpl" }}
    {{- tpl ( $fetch_base_pkgs ) $tpl_context | nindent 0 }}
  {{- end }}


  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
run_log_output
  {{- end }}

#run phase2
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
