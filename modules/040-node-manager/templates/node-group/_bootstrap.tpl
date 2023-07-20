{{- define "bootstrap_script" -}}
#!/usr/bin/env bash

set -Eeuo pipefail

function detect_bundle() {
  {{- .Files.Get "candi/bashible/detect_bundle.sh" | nindent 2 }}
}

bundle="$(detect_bundle)"

function install_jq() {
  case "$1" in
    ubuntu-lts|debian|altlinux|astra)
      apt-get update && apt-get install jq -y
      ;;
    alteros|redos)
      yum updateinfo && yum install jq -y
      ;;
    centos)
      yum install epel-release -y && yum updateinfo && yum install jq -y
    *)
      echo "Unsupported bundle $1 for bootstrap.sh! Exiting..."
      exit 1
      ;;
  esac
}

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

until install_jq "$bundle"; do
  echo "Error installing jq pacage"
  sleep 10
done

bootstrap_object="$(get_bootstrap)"

bash <<< "$(echo "$bootstrap_object" | jq -r .bootstrap)"
{{- end }}
