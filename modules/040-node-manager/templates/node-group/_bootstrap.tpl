{{- define "bootstrap_script" -}}
#!/usr/bin/env bash

set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

bundle="$(detect_bundle)"

function get_bootstrap() {
  token="$(</var/lib/bashible/bootstrap-token)"
  node_group_name="{{ .nodeGroupName }}"

  bootstrap_bundle_name="$bundle.$node_group_name"

  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/bootstrap/$bootstrap_bundle_name"
      if curl -s -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "/var/lib/bashible/ca.crt"
      then
        return 0
      else
        >&2 echo "failed to get bootstrap $bootstrap_bundle_name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

bootstrap_object="$(get_bootstrap)"
export bootstrap_object

if ! bootstrap_script="$(python <<"EOF"
from __future__ import print_function
import json
import os

data = json.loads(os.environ['bootstrap_object'])
print(data["bootstrap"])
EOF
)"; then
  echo "Failed to get bootstrap script, exiting..."
fi

bash <<< "$bootstrap_script"
{{- end }}
