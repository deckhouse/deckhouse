#!/usr/bin/env bash

function get_bundle() {
  resource="$1"
  name="$2"
  token="$(</var/lib/bashible/bootstrap-token)"

  while true; do
    for server in {{ .normal.apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"
      if curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
       return 0
      else
        >&2 echo "failed to get $resource $name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

function basic_bootstrap_{{ .bundle }} {
  {{- tpl (.Files.Get (printf "/deckhouse/candi/bashible/bundles/%s/bootstrap.sh.tpl" .bundle)) . | nindent 2 }}
}

set -Eeuo pipefail
shopt -s failglob

BOOTSTRAP_DIR="/var/lib/bashible"
TMPDIR="/opt/deckhouse/tmp"
mkdir -p "$BOOTSTRAP_DIR" "$TMPDIR"

# Directory contains sensitive information
chmod 0700 $BOOTSTRAP_DIR

# Temporary dir
export TMPDIR=/opt/deckhouse/tmp
mkdir -p "$TMPDIR"

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

# Install necessary packages.
basic_bootstrap_${BUNDLE}

bootstrap_job_log_pid=""

  {{- if eq .nodeGroup.nodeType "CloudEphemeral" }}
# Put bootstrap log information to Machine resource status
patch_pending=true
output_log_port=8000
while [ "$patch_pending" = true ] ; do
  for server in {{ .normal.apiserverEndpoints | join " " }} ; do
    server_addr=$(echo $server | cut -f1 -d":")
    until tcp_endpoint="$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9\.]+)')"; do
      echo "The network is not ready for connecting to apiserver yet, waiting..."
      sleep 1
    done

    if curl -sS --fail -x "" \
      --max-time 10 \
      -XPATCH \
      -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
      -H "Accept: application/json" \
      -H "Content-Type: application/json-patch+json" \
      --cacert "$BOOTSTRAP_DIR/ca.crt" \
      --data "[{\"op\":\"add\",\"path\":\"/status/bootstrapStatus\", \"value\": {\"description\": \"Use 'nc ${tcp_endpoint} ${output_log_port}' to get bootstrap logs.\", \"logsEndpoint\": \"${tcp_endpoint}:${output_log_port}\"} }]" \
      "https://$server/apis/deckhouse.io/v1alpha1/instances/$(hostname -s)/status" ; then

      echo "Successfully patched machine $(hostname -s) status."
      patch_pending=false

      break
    else
      >&2 echo "Failed to patch machine $(hostname -s) status."
      sleep 10
      continue
    fi
  done
done

# Start output bootstrap logs
if type socat >/dev/null 2>&1; then
  socat -u FILE:/var/log/cloud-init-output.log,ignoreeof TCP4-LISTEN:8000,fork,reuseaddr &
  bootstrap_job_log_pid=$!
else
  while true; do cat /var/log/cloud-init-output.log | nc -l "$tcp_endpoint" "$output_log_port"; done &
  bootstrap_job_log_pid=$!
fi

  {{- end }}

# IMPORTANT !!! Centos/Redhat put jq in /usr/local/bin but it is not in PATH.
export PATH="/opt/deckhouse/bin:$PATH"
# Get bashible script from secret
get_bundle bashible "${BUNDLE}.{{ .nodeGroup.name }}" | jq -r '.data."bashible.sh"' > $BOOTSTRAP_DIR/bashible.sh
chmod +x $BOOTSTRAP_DIR/bashible.sh

# Bashible first run
until /var/lib/bashible/bashible.sh; do
  echo "Error running bashible script. Retry in 10 seconds."
  sleep 10
done;

# Stop output bootstrap logs
if [ -n "${bootstrap_job_log_pid-}" ] && kill -s 0 "${bootstrap_job_log_pid-}" 2>/dev/null; then
  kill -9 "${bootstrap_job_log_pid-}"
fi
