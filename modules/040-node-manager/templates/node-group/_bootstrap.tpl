{{- define "bootstrap_script" -}}
#!/usr/bin/env bash

set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

bundle="$(detect_bundle)"
token="$(</var/lib/bashible/bootstrap-token)"
node_group_name="{{ .nodeGroupName }}"
bootstrap_bundle_name="$bundle.$node_group_name"
url="https://$server/apis/bashible.deckhouse.io/v1alpha1/bootstrap/$bootstrap_bundle_name"

python_binary=""
http_client_binary=""

if command -v python3 >/dev/null 2>&1; then
  python_binary="python3"
elif command -v python2 >/dev/null 2>&1; then
  python_binary="python2"
elif command -v python >/dev/null 2>&1; then
  python_binary="python"
else
  echo "Python not found, exiting..."
  exit 1
fi

if command -v curl >/dev/null 2>&1; then
  http_client_binary="curl"
elif command -v wget >/dev/null 2>&1; then
  http_client_binary="wget"
else
  echo "HTTP client binary not found, exiting..."
  exit 1
fi

function get_bootstrap_curl() {
  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
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

function get_bootstrap_wget() {
  while true; do
    for server in {{ .apiserverEndpoints | join " " }}; do
      if wget -qvn -O - --header="Authorization: Bearer $token" --ca-certificate="/var/lib/bashible/ca.crt" "$url"
      then
        return 0
      else
        >&2 echo "failed to get bootstrap $bootstrap_bundle_name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

bootstrap_object=""

if [ "$http_client_binary" == "curl" ]; then
  bootstrap_object="$(get_bootstrap_curl)"
elif [ "$http_client_binary" == "wget" ]; then
  bootstrap_object="$(get_bootstrap_wget)"
else
  echo "Invalid http_client_binary value, exiting..."
  exit 1
fi

export bootstrap_object

if ! bootstrap_script="$("$python_binary" <<"EOF"
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
