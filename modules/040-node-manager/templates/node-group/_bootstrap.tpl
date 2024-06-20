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


	{{- if $base_pkgs_source := $context.Files.Get "candi/bashible/base_pkgs_source.sh.tpl" }}
cat > ${TMPDIR}/base_pkgs_source.sh <<"EOF"
	{{- tpl $base_pkgs_source $tpl_context | nindent 0 }}
EOF
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

function prepare_base_d8_binaries() {
  {{- with $context.Values.global.modulesImages.digests.registrypackages }}
  bb-package-install "jq:{{ .jq16 }}" "curl:{{ .d8Curl821 }}" "netcat:{{ .netcat110481 }}"
  {{- end }}
}

  {{- if not (hasKey $ng "staticInstances") }}
    {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
function run_cloud_network_setup() {
      {{- if $bootstrap_script_common := $context.Files.Get "candi/bashible/bootstrap-networks.sh.tpl" }}
  cat > ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh <<"EOF"
      {{- tpl $bootstrap_script_common $tpl_context | nindent 0 }}
EOF
  chmod +x ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh
      {{- end }}
  {{- /*
  # Execute cloud provider specific network bootstrap script. It will organize connectivity to kube-apiserver.
  */}}

  if [[ -f ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh ]] ; then
    until ${BOOTSTRAP_DIR}/cloud-provider-bootstrap-networks.sh; do
      >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
      sleep 10
    done
  fi
}
    {{- end }}
  {{- end }}
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
function run_log_output() {
  if type nc >/dev/null 2>&1; then
    tail -n 100 -f ${TMPDIR}/bootstrap.log | nc -l -p 8000 &
    bootstrap_job_log_pid=$!
  fi
}
  {{- end }}

source ${TMPDIR}/base_pkgs_source.sh
prepare_base_d8_binaries
  {{- if not (hasKey $ng "staticInstances") }}
    {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
run_cloud_network_setup
    {{- end }}
  {{- end }}
rm -f ${TMPDIR}/base_pkgs_source.sh
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
