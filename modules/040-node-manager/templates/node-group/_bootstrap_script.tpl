{{- define "node_group_bashible_bootstrap_script" -}}
  {{- $context := . -}}

  {{- include "node_group_bashible_bootstrap_script_base_bootstrap" $context }}

bootstrap_job_log_pid=""

  {{- if eq .nodeGroup.nodeType "CloudEphemeral" }}
    {{- /*
# Put bootstrap log information to Machine resource status
    */}}
patch_pending=true
output_log_port=8000
while [ "$patch_pending" = true ] ; do
  for server in {{ .normal.apiserverEndpoints | join " " }} ; do
    server_addr=$(echo $server | cut -f1 -d":")
    until tcp_endpoint="$(ip ro get ${server_addr} | grep -Po '(?<=src )([0-9\.]+)')"; do
      echo "The network is not ready for connecting to apiserver yet, waiting..."
      sleep 1
    done

    if curl -s --fail \
      --max-time 10 \
      -XPATCH \
      -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
      -H "Accept: application/json" \
      -H "Content-Type: application/json-patch+json" \
      --cacert "$BOOTSTRAP_DIR/ca.crt" \
      --data "[{\"op\":\"add\",\"path\":\"/status/bootstrapStatus\", \"value\": {\"description\": \"Use 'nc ${tcp_endpoint} ${output_log_port}' to get bootstrap logs.\", \"tcpEndpoint\": \"${tcp_endpoint}\"} }]" \
      "https://$server/apis/machine.sapcloud.io/v1alpha1/namespaces/d8-cloud-instance-manager/machines/$(hostname -s)/status" ; then

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

    {{- /*
# Start output bootstrap logs
    */}}
if type socat >/dev/null 2>&1; then
  socat -u FILE:/var/log/cloud-init-output.log,ignoreeof TCP4-LISTEN:8000,fork,reuseaddr &
  bootstrap_job_log_pid=$!
else
  while true; do cat /var/log/cloud-init-output.log | nc -l "$tcp_endpoint" "$output_log_port"; done &
  bootstrap_job_log_pid=$!
fi

  {{- end }}

  {{- include "node_group_bashible_bootstrap_script_download_bashible" $context }}

    {{- /*
# Bashible first run
    */}}
until /var/lib/bashible/bashible.sh; do
  echo "Error running bashible script. Retry in 10 seconds."
  sleep 10
done;

    {{- /*
# Stop output bootstrap logs
    */}}
if [ -n "${bootstrap_job_log_pid-}" ] && kill -s 0 "${bootstrap_job_log_pid-}" 2>/dev/null; then
  kill -9 "${bootstrap_job_log_pid-}"
fi

{{- end }}

{{- define "node_group_bashible_bootstrap_script_noninteractive" -}}
  {{- $context := . -}}

  {{- include "node_group_bashible_bootstrap_script_base_bootstrap" $context }}

  {{- include "node_group_bashible_bootstrap_script_download_bashible" $context }}

cat > /etc/systemd/system/bashible.timer << "EOF"
[Unit]
Description=bashible timer

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/bashible.service << "EOF"
[Unit]
Description=Bashible service

[Service]
EnvironmentFile=/etc/environment
ExecStart=/var/lib/bashible/bashible.sh --max-retries 10
EOF

systemctl daemon-reload
systemctl restart bashible.timer
systemctl enable bashible.timer
{{- end }}

{{- define "node_group_bashible_bootstrap_script_base_bootstrap" -}}
  {{- $context := . -}}
#!/bin/bash

function get_bundle() {
  resource="$1"
  name="$2"
  token="$(</var/lib/bashible/bootstrap-token)"

  while true; do
    for server in {{ .normal.apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/${resource}s/${name}"
      if curl -s -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
       return 0
      else
        >&2 echo "failed to get $resource $name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

function detect_bundle() {
  {{- tpl ($context.Files.Get "candi/bashible/detect_bundle.sh") $context | nindent 2 }}
}

  {{- range $bundle := $context.allowedBundles }}
function basic_bootstrap_{{ $bundle }} {
  {{- tpl ($context.Files.Get (printf "candi/bashible/bundles/%s/bootstrap.sh.tpl" $bundle)) $context | nindent 2 }}
}
  {{ end }}

set -Eeuo pipefail
shopt -s failglob

BOOTSTRAP_DIR="/var/lib/bashible"
mkdir -p $BOOTSTRAP_DIR

  {{- /*
# Directory contains sensitive information
  */}}
chmod 0700 $BOOTSTRAP_DIR

  {{- /*
# Detect bundle
  */}}
BUNDLE="$(detect_bundle)"

  {{- /*
# Install necessary packages. Not in cloud config because cloud init do not retry installation and silently fails.
  */}}
basic_bootstrap_${BUNDLE}

  {{- /*
# Execute cloud provider specific network bootstrap script. It will organize connectivity to kube-apiserver.
  */}}
if [[ -f $BOOTSTRAP_DIR/cloud-provider-bootstrap-networks.sh ]] ; then
  until $BOOTSTRAP_DIR/cloud-provider-bootstrap-networks.sh; do
    >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
    sleep 10
  done
elif [[ -f $BOOTSTRAP_DIR/cloud-provider-bootstrap-networks-${BUNDLE}.sh ]] ; then
  until $BOOTSTRAP_DIR/cloud-provider-bootstrap-networks-${BUNDLE}.sh; do
    >&2 echo "Failed to execute cloud provider specific bootstrap. Retry in 10 seconds."
    sleep 10
  done
fi
{{- end }}

{{- define "node_group_bashible_bootstrap_script_download_bashible" -}}
  {{- $context := . }}
  {{- /*
# IMPORTANT !!! Centos/Redhat put jq in /usr/local/bin but it is not in PATH.
  */}}
export PATH="/usr/local/bin:$PATH"
  {{- /*
# Get bashible script from secret
  */}}
get_bundle bashible "${BUNDLE}.{{ .nodeGroup.name }}" | jq -r '.data."bashible.sh"' > $BOOTSTRAP_DIR/bashible.sh
chmod +x $BOOTSTRAP_DIR/bashible.sh
{{- end }}
