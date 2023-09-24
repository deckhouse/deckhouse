{{- define "bootstrap_script" }}
#!/usr/bin/env bash
set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

function p3_script() {
  cat - <<EOF
import sys
import requests
import json
response = requests.get(sys.argv[1], headers={'Authorization': 'Bearer ' + sys.argv[2]}, verify='/var/lib/bashible/ca.crt')
data = json.loads(response.content)
sys.stdout.write(data["bootstrap"])
EOF
}

function get_phase2() {
  bundle="$(detect_bundle)"
  bootstrap_bundle_name="$bundle.{{ .nodeGroupName }}"
  token="$(</var/lib/bashible/bootstrap-token)"
  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
      url="https://$server/apis/bashible.deckhouse.io/v1alpha1/bootstrap/$bootstrap_bundle_name"
      # Try curl
      if curl -sS -f -x "" -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"; then
        return 0
      fi
      # fallback to python3
      if python3 - "$url" "$token" <<< "$(p3_script)"; then
        return 0
      fi
      >&2 echo "failed to get bootstrap $bootstrap_bundle_name from $url"
    done
    sleep 10
  done
}

get_phase2 | bash
{{- end }}
