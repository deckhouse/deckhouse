{{- define "bootstrap_script" }}
#!/bin/bash

set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "/bashible/templates/bashible/detect_bundle.sh" | nindent 2 }}
}

function get_bootstrap() {
  token="$(</var/lib/bashible/bootstrap-token)"
  bundle="$(detect_bundle)"
  node_group_name="{{ .nodeGroupName }}"

  bootstrap_bundle_name="$bundle-$node_group_name"

  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/bootstrap/$bootstrap_bundle_name"
      if curl -s -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
        return 0
      else
        >&2 echo "failed to get bootstrap $bootstrap_bundle_name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

bash <<< "$(get_bootstrap)"
{{- end }}
