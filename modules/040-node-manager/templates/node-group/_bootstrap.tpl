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

function install_curl() {
  case "$1" in
    altlinux|astra)
      export DEBIAN_FRONTEND=noninteractive
      apt-get update && apt-get install curl -y
      ;;
  esac
}

until install_curl "$bundle"; do
  echo "Error installing curl package"
  sleep 10
done

bootstrap_object="$(get_bootstrap)"
export bootstrap_object

python_binary=""

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
