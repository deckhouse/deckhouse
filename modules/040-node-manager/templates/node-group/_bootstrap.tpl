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
    from urllib.request import urlopen, Request, HTTPError
except ImportError as e:
    from urllib2 import urlopen, Request, HTTPError

ssl.match_hostname = lambda cert, hostname: True
request = Request(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]})
try:
    # Try with context first (Python 3.13+), fallback to cafile for older versions
    try:
        # Create SSL context for Python 3.13 compatibility
        context = ssl.create_default_context()
        context.load_verify_locations('/var/lib/bashible/ca.crt')
        response = urlopen(request, context=context, timeout=10)
    except TypeError:
        # Fallback for older Python versions that don't support context parameter
        response = urlopen(request, cafile='/var/lib/bashible/ca.crt', timeout=10)
    data = json.loads(response.read())
    sys.stdout.write(data["bootstrap"])
except HTTPError as e:
    if e.getcode() == 401:
        sys.stderr.write("Bootstrap-token compiled in this bootstrap.sh script is expired. Looks like more than 4 hours passed from the time it's been issued.\n")
        sys.exit(2)
    sys.stderr.write("Access to {} returned HTTP Error {}: {}\n".format(sys.argv[1], e.getcode(), e.read()[:255]))
    sys.exit(1)
except Exception as e:
    sys.stderr.write("Error fetching bootstrap from {}: {}\n".format(sys.argv[1], e))
    sys.exit(1)
EOF
}

function get_phase2() {
  bootstrap_ng_name="{{ $ng.name }}"
  token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  check_python

  local http_401_count=0
  local max_http_401_count=6

  while true; do
    for server in {{ $context.Values.nodeManager.internal.clusterMasterAddresses | join " " }}; do
      url="https://${server}/apis/bashible.deckhouse.io/v1alpha1/bootstrap/${bootstrap_ng_name}"
      if eval "${python_binary}" - "${url}" "${token}" <<< "$(load_phase2_script)"; then
        return 0
      fi
      local rc=$?
      if [ $rc -eq 2 ]; then
        ((http_401_count++))
        if [ $http_401_count -ge $max_http_401_count ]; then
          return 1
        fi
      else
        >&2 echo "failed to get bootstrap ${bootstrap_ng_name} from $url (exit code $rc)"
      fi
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
