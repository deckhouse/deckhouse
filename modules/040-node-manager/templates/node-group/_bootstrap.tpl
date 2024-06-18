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


function check_python() {
  for pybin in python3 python2 python; do
    if command -v "$pybin" >/dev/null 2>&1; then
      python_binary="$pybin"
      return 0
    fi
  done
  echo "Python not found, exiting..."
  return 1
}

bb-package-install() {
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"
    bb-package-fetch "${PACKAGE_WITH_DIGEST}"
    local TMP_DIR=""
    TMP_DIR="$(mktemp -d)"
    tar -xf "${BB_FETCHED_PACKAGES_STORE}/${PACKAGE}/${DIGEST}.tar.gz" -C "${TMP_DIR}"

    # shellcheck disable=SC2164
    pushd "${TMP_DIR}" >/dev/null
    ./install
    popd >/dev/null
    mkdir -p "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    echo "${DIGEST}" > "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}/digest"
    cp "${TMP_DIR}/install" "${TMP_DIR}/uninstall" "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    rm -rf "${TMP_DIR}" "${BB_FETCHED_PACKAGES_STORE:?}/${PACKAGE}"
  done
}

bb-package-fetch() {
  mkdir -p "${BB_FETCHED_PACKAGES_STORE}"
  declare -A PACKAGES_MAP
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"
    PACKAGES_MAP[$DIGEST]="${PACKAGE}"
  done
  bb-package-fetch-blobs PACKAGES_MAP
}

bb-package-fetch-blobs() {
  local PACKAGE_DIGEST
  for PACKAGE_DIGEST in "${!PACKAGES_MAP[@]}"; do
    local PACKAGE_DIR="${BB_FETCHED_PACKAGES_STORE}/${PACKAGES_MAP[$PACKAGE_DIGEST]}"
    mkdir -p "${PACKAGE_DIR}"
    bb-package-fetch-blob "${PACKAGE_DIGEST}" "${PACKAGE_DIR}/${PACKAGE_DIGEST}.tar.gz"
  done
}

bb-package-fetch-blob() {
  check_python

  cat - <<EOF | $python_binary
import random
import ssl
try:
    from urllib.request import urlopen, Request
except ImportError as e:
    from urllib2 import urlopen, Request
# Choose a random endpoint to increase fault tolerance and reduce load on a single endpoint.
endpoints = "${PACKAGES_PROXY_ADDRESSES}".split(",")
endpoint = random.choice(endpoints)
ssl._create_default_https_context = ssl._create_unverified_context
url = 'https://{}/package?digest=$1&repository=${REPOSITORY}'.format(endpoint)
request = Request(url, headers={'Authorization': 'Bearer ${PACKAGES_PROXY_TOKEN}'})
response = urlopen(request, timeout=300)
with open('$2', 'wb') as f:
    f.write(response.read())
EOF
}

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
  bootstrap_bundle_name="${BUNDLE}.{{ $ng.name }}"
  token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  while true; do
    for server in {{ $context.Values.nodeManager.internal.clusterMasterAddresses | join " " }}; do
      url="https://${server}/apis/bashible.deckhouse.io/v1alpha1/bootstrap/${bootstrap_bundle_name}"
      if eval "${python_binary}" - "${url}" "${token}" <<< "$(load_phase2_script)"; then
        return 0
      fi
      >&2 echo "failed to get bootstrap ${bootstrap_bundle_name} from $url"
    done
    sleep 10
  done
}

function prepare_base_d8_binaries() {
  export PATH="/opt/deckhouse/bin:$PATH"
  export LANG=C
  export REPOSITORY=""
  export BB_INSTALLED_PACKAGES_STORE="/var/cache/registrypackages"
  export BB_FETCHED_PACKAGES_STORE="/${TMPDIR}/registrypackages"
{{- with $context.Values.global.clusterConfiguration }}
{{- if .proxy }}
    {{- if .proxy.httpProxy }}
  export HTTP_PROXY={{ .proxy.httpProxy | quote }}
  export http_proxy=${HTTP_PROXY}
    {{- end }}
    {{- if .proxy.httpsProxy }}
  export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
  export https_proxy=${HTTPS_PROXY}
    {{- end }}
    {{- if .proxy.noProxy }}
  export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
  export no_proxy=${NO_PROXY}
    {{- end }}
  {{- else }}
  unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
{{- end }}
  {{- if $context.Values.nodeManager.internal.packagesProxy }}
  export PACKAGES_PROXY_ADDRESSES="{{ $context.Values.nodeManager.internal.packagesProxy.addresses | join "," }}"
  export PACKAGES_PROXY_TOKEN="{{ $context.Values.nodeManager.internal.packagesProxy.token }}"
  {{- end }}
{{- with $context.Values.global.modulesImages.digests.registrypackages }}
  bb-package-install "jq:{{ .jq16 }}" "curl:{{ .d8Curl821 }}" "netcat:{{ .netcat110481 }}"
{{- end }}
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
  {{- /*
  # Start output bootstrap logs
  */}}
  if type nc >/dev/null 2>&1; then
    tail -n 100 -f ${TMPDIR}/bootstrap.log | nc -l -p 8000 &
    bootstrap_job_log_pid=$!
  fi
}
 {{- end }}
  {{- if not (hasKey $ng "staticInstances") }}
  {{- if hasKey $context.Values.nodeManager.internal "cloudProvider" }}
run_cloud_network_setup
  {{- end }}
  {{- end }}
  {{- if or (eq $ng.nodeType "CloudEphemeral") (hasKey $ng "staticInstances") }}
run_log_output
  {{- end }}
prepare_base_d8_binaries
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
