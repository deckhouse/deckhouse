{{- define "node_group_bashible_bootstrap_script" -}}
  {{- $context := . -}}
#!/bin/bash

function get_secret() {
  secret="$1"

  while true; do
    for server in {{ .normal.apiserverEndpoints | join " " }}; do
      if curl -s -f -X GET "https://$server/api/v1/namespaces/d8-cloud-instance-manager/secrets/$secret" --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
        return 0
      else
        >&2 echo "failed to get secret $secret with curl https://$server..."
      fi
    done
    sleep 10
  done
}

function detect_bundle() {
{{ tpl ($context.Files.Get "candi/bashible/detect_bundle.sh") $context | indent 2 }}
}

  {{ range $bundle := list "ubuntu-lts" "ubuntu-18.04" "centos-7" }}
function basic_bootstrap_{{ $bundle }} {
{{- if eq $bundle "ubuntu-18.04" }}
{{ tpl ($context.Files.Get (printf "candi/bashible/bundles/%s/bootstrap.sh.tpl" "ubuntu-lts")) $context | indent 2 }}
{{- else }}
{{ tpl ($context.Files.Get (printf "candi/bashible/bundles/%s/bootstrap.sh.tpl" $bundle)) $context | indent 2 }}
{{- end }}
}
  {{ end }}

set -Eeuo pipefail
shopt -s failglob

BOOTSTRAP_DIR="/var/lib/bashible"
mkdir -p $BOOTSTRAP_DIR

# Directory contains sensitive information
chmod 0700 $BOOTSTRAP_DIR

# Detect bundle
BUNDLE="$(detect_bundle)"

#Install necessary packages. Not in cloud config cause cloud init do not retry installation and silently fails.
basic_bootstrap_${BUNDLE}

# Execute cloud provider specific network bootstrap script. It will organize connectivity to kube-apiserver.
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

  {{- if eq .nodeGroup.nodeType "Cloud" }}
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

    if curl -s --fail \
      --max-time 10 \
      -XPATCH \
      -H "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)" \
      -H "Accept: application/json" \
      -H "Content-Type: application/json-patch+json" \
      --cacert "$BOOTSTRAP_DIR/ca.crt" \
      --data "[{\"op\":\"add\",\"path\":\"/status/bootstrapStatus\", \"value\": {\"description\": \"Use 'nc ${tcp_endpoint} ${output_log_port}' to get bootstrap logs.\", \"tcpEndpoint\": \"${tcp_endpoint}\"} }]" \
      "https://$server/apis/machine.sapcloud.io/v1alpha1/namespaces/d8-cloud-instance-manager/machines/$(hostname)/status" ; then

      echo "Successfully patched machine $(hostname) status."
      patch_pending=false

      break
    else
      >&2 echo "Failed to patch machine $(hostname) status."
      sleep 10
      continue
    fi
  done
done

# Start output bootstrap logs
while true; do cat /var/log/cloud-init-output.log | nc -l "$tcp_endpoint" "$output_log_port"; done &
  {{- end }}

# Get bashible script from secret
get_secret bashible-{{ .nodeGroup.name }}-${BUNDLE} | jq -r '.data."bashible.sh"' | base64 -d > $BOOTSTRAP_DIR/bashible.sh
chmod +x $BOOTSTRAP_DIR/bashible.sh

# Bashible first run
until /var/lib/bashible/bashible.sh; do
  echo "Error running bashible script. Retry in 10 seconds."
  sleep 10
done;

# Stop output bootstrap logs
kill -9 %1
{{- end }}
