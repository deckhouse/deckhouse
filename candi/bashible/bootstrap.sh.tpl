#!/usr/bin/env bash

function get_bundle() {
  resource="$1"
  name="$2"
  token="$(</var/lib/bashible/bootstrap-token)"

  while true; do
    for server in {{ .normal.apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"
      if d8-curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
       return 0
      else
        >&2 echo "failed to get $resource $name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

set -Eeuo pipefail
shopt -s failglob

export BOOTSTRAP_DIR="/var/lib/bashible"
export TMPDIR="/opt/deckhouse/tmp"
mkdir -p "$BOOTSTRAP_DIR" "$TMPDIR"

# Directory contains sensitive information
chmod 0700 $BOOTSTRAP_DIR

# Detect bundle
BUNDLE="{{ .bundle }}"

# set proxy env variables
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
{{- if and (ne .nodeGroup.nodeType "Static") (ne .nodeGroup.nodeType "CloudStatic" )}}
export D8_NODE_HOSTNAME=$(hostname -s)
{{- else }}
export D8_NODE_HOSTNAME=$(hostname)
{{- end }}

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
  for server in {{ .normal.apiserverEndpoints | join " " }} ; do
    server_addr=$(echo $server | cut -f1 -d":")
    until tcp_endpoint="$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9\.]+)')"; do
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
      --data "[{\"op\":\"add\",\"path\":\"/status/bootstrapStatus\", \"value\": {\"description\": \"Use 'nc ${tcp_endpoint} ${output_log_port}' to get bootstrap logs.\", \"logsEndpoint\": \"${tcp_endpoint}:${output_log_port}\"} }]" \
      "https://$server/apis/deckhouse.io/v1alpha1/instances/${machine_name}/status" ; then

      echo "Successfully patched instance ${machine_name} status."
      patch_pending=false

      break
    else
      failure_count=$((failure_count + 1))

      if [[ $failure_count -eq $failure_limit ]]; then
        >&2 echo "Failed to patch instance ${machine_name} status. Number of attempts exceeded. Status patch will be skipped."
        patch_pending=false
        break
      fi

      >&2 echo "Failed to patch instance ${machine_name} status. ${failure_count} of ${failure_limit} attempts..."
      sleep 10
      continue
    fi
  done
done
{{- end }}

export PATH="/opt/deckhouse/bin:$PATH"
# Get bashible script from secret
get_bundle bashible "${BUNDLE}.{{ .nodeGroup.name }}" | jq -r '.data."bashible.sh"' > $BOOTSTRAP_DIR/bashible.sh
chmod +x $BOOTSTRAP_DIR/bashible.sh

# Bashible first run
until /var/lib/bashible/bashible.sh; do
  echo "Error running bashible script. Retry in 10 seconds."
  sleep 10
done
