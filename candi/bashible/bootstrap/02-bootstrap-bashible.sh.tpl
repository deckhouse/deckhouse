#!/usr/bin/env bash
{{- /*
# Copyright 2024 Flant JSC
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
*/}}

# candi/bashible/bootstrap/02-bootstrap-bashible.sh.tpl

set -Eeuo pipefail
shopt -s failglob

export BOOTSTRAP_DIR="/var/lib/bashible"
export PATH="/opt/deckhouse/bin:$PATH"
export TMPDIR="/opt/deckhouse/tmp"
mkdir -p "$BOOTSTRAP_DIR" "$TMPDIR"

# Directory contains sensitive information
chmod 0700 $BOOTSTRAP_DIR

unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy

bootstrap_log_init() {
  if [[ -z ${BOOTSTRAP_LOG_INITIALIZED:-} ]]; then
    mkdir -p /var/log/d8/bashible
    exec {bootstrap_stdout_fd}>&1
    exec > >(tee -a /var/log/d8/bashible/bootstrap.log >&${bootstrap_stdout_fd}) 2>&1
    export BOOTSTRAP_LOG_INITIALIZED=1
  fi
}

bootstrap_log_init

{{- $candi := "candi/bashible/lib.sh.tpl" -}}
{{- $deckhouse := "/deckhouse/candi/bashible/lib.sh.tpl" -}}
{{- $lib := .Files.Get $deckhouse | default (.Files.Get $candi) -}}
{{- $ctx := . -}}
{{- tpl (printf `
%s

{{ template "bb-d8-node-name" $ }}
{{ template "bb-discover-node-name" $ }}

` $lib) $ctx }}

bb-discover-node-name
export D8_NODE_HOSTNAME=$(bb-d8-node-name)

function get_bundle() {
  resource="$1"
  name="$2"
  token="$(</var/lib/bashible/bootstrap-token)"

  while true; do
    for server in {{ .clusterMasterKubeAPIEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"
      if d8-curl -sS -f -x "" --connect-timeout 10 -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
       return 0
      else
        >&2 echo "Failed to get $resource $name from $server"
      fi
    done
    sleep 10
  done
}

{{- if or (eq .nodeGroup.nodeType "CloudEphemeral") (hasKey .nodeGroup "staticInstances") }}
# Put bootstrap log information to Machine resource status if it is a cloud installation or cluster-api static machine
patch_pending=true
output_log_port=8000
# Skip this step after multiple failures.
# This step puts information "how to get bootstrap logs" into Instance resource.
# It's not critical, and waiting for it indefinitely, breaking bootstrap, is not reasonable.
failure_count=0
failure_limit=3
while [ "$patch_pending" = true ] ; do
  for server in {{ .clusterMasterKubeAPIEndpoints | join " " }} ; do
    # Handle IPv6 endpoints like [fe80::1]:6443 or IPv4 like 10.0.0.1:6443
    if [[ "$server" == \[*\]:* ]]; then
      server_addr="${server#[}"
      server_addr="${server_addr%%]:*}"
    else
      server_addr="${server%:*}"
    fi
    until node_ip="$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9a-fA-F\.:]+)')"; do
      echo "The network is not ready for connecting to apiserver yet, waiting..."
      sleep 1
    done

    machine_name="${D8_NODE_HOSTNAME}"
    if [ -f ${BOOTSTRAP_DIR}/machine-name ]; then
      machine_name="$(<${BOOTSTRAP_DIR}/machine-name)"
    fi

    if d8-curl -sS --fail -x "" \
      --max-time 10 \
      -XPATCH \
      -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
      -H "Accept: application/json" \
      -H "Content-Type: application/json-patch+json" \
      --cacert "$BOOTSTRAP_DIR/ca.crt" \
      --data "[{\"op\":\"add\",\"path\":\"/status/bootstrapStatus\", \"value\": {\"description\": \"Use curl -N 'http://${node_ip}:${output_log_port}' to get bootstrap logs.\", \"logsEndpoint\": \"http://${node_ip}:${output_log_port}\"} }]" \
      "https://$server/apis/deckhouse.io/v1alpha2/instances/${machine_name}/status" ; then

      echo "Patched instance ${machine_name} status via $server"
      patch_pending=false

      break
    else
      failure_count=$((failure_count + 1))

      if [[ $failure_count -eq $failure_limit ]]; then
        >&2 echo "Failed to patch instance ${machine_name} status, retry limit reached, skipping status patch"
        patch_pending=false
        break
      fi

      >&2 echo "Failed to patch instance ${machine_name} status (attempt ${failure_count} of ${failure_limit})"
      sleep 10
      continue
    fi
  done
done
{{- end }}

# Get bashible script from secret
get_bundle bashible "{{ .nodeGroup.name }}" | jq -r '.data."bashible.sh"' > $BOOTSTRAP_DIR/bashible.sh
chmod +x $BOOTSTRAP_DIR/bashible.sh

# Bashible first run
until bash --noprofile --norc -c /var/lib/bashible/bashible.sh; do
  echo "bashible script failed, retrying in 10 seconds"
  sleep 10
done
