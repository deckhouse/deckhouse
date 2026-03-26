
{{- define "bb-d8-node-name" -}}
bb-d8-node-name() {
  echo $(</var/lib/bashible/discovered-node-name)
}
{{- end }}


{{- define "bb-d8-machine-name" -}}
bb-d8-machine-name() {
  local bootstrap_dir="${BOOTSTRAP_DIR:-/var/lib/bashible}"
  local machine_name_file="${bootstrap_dir}/machine-name"

  if [ -s "$machine_name_file" ]; then
    echo "$(<"$machine_name_file")"
    return 0
  fi

  bb-d8-node-name
}
{{- end }}


{{- define "bb-d8-node-ip" -}}
bb-d8-node-ip() {
  echo $(</var/lib/bashible/discovered-node-ip)
}
{{- end }}


{{- define "bb-discover-node-name" -}}
bb-discover-node-name() {
  local discovered_name_file="/var/lib/bashible/discovered-node-name"
  local kubelet_crt="/var/lib/kubelet/pki/kubelet-server-current.pem"

  if [ ! -s "$discovered_name_file" ]; then
    if [[ -s "$kubelet_crt" ]]; then
      openssl x509 -in "$kubelet_crt" \
        -noout -subject -nameopt multiline |
      awk '/^ *commonName/{print $NF}' | cut -d':' -f3- > "$discovered_name_file"
    else
    {{- if and (ne .nodeGroup.nodeType "Static") (ne .nodeGroup.nodeType "CloudStatic") }}
      if [[ "$(hostname)" != "$(hostname -s)" ]]; then
        hostnamectl set-hostname "$(hostname -s)"
      fi
    {{- end }}
      hostname > "$discovered_name_file"
    fi
  fi
}
{{- end }}


{{- define "bb-minget" -}}
bb-minget-install() {
  local minget_path="/opt/deckhouse/bin/minget"
  local minget_b64='{{ .mingetB64 }}'

  if [[ -f "${minget_path}" ]]; then
    return 0
  fi

  mkdir -p /opt/deckhouse/bin/
  printf '%s' "${minget_b64}" | base64 -d > "${minget_path}"
  chmod +x "${minget_path}"
}

bb-rpp-get-install() {
  local rpp_client_path="/opt/deckhouse/bin/rpp-get"
  local rpp_client_digest="{{ .images.registrypackages.rppGet }}"
  local bootstrap_cluster_uuid="${PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID}"
  local bootstrap_path_prefix=""
  local installed_store="${BB_RP_INSTALLED_PACKAGES_STORE:-/var/cache/registrypackages}"
  local rpp_client_store="${installed_store}/rpp-get"
  local digest_path="${rpp_client_store}/digest"

  if [[ -n "${bootstrap_cluster_uuid}" ]]; then
    bootstrap_path_prefix="/${bootstrap_cluster_uuid}"
  fi

  if [[ -x "${rpp_client_path}" ]] &&
     [[ -f "${digest_path}" ]] &&
     [[ "$(<"${digest_path}")" == "${rpp_client_digest}" ]]; then
    return 0
  fi

  if [[ -z "${PACKAGES_PROXY_BOOTSTRAP_ADDRESSES:-}" ]]; then
    >&2 echo "rpp-get bootstrap source is not configured"
    return 1
  fi

  mkdir -p "${rpp_client_path%/*}" "${rpp_client_store}"

  local tmp_path="${rpp_client_path}.tmp"
  local address

  while true; do
    for address in ${PACKAGES_PROXY_BOOTSTRAP_ADDRESSES}; do
      rm -f "${tmp_path}"
      if /opt/deckhouse/bin/minget "${address}${bootstrap_path_prefix}/rpp-get?digest=${rpp_client_digest}" > "${tmp_path}"; then
        chmod +x "${tmp_path}"
        if "${tmp_path}" version >/dev/null 2>&1; then
          mv -f "${tmp_path}" "${rpp_client_path}"
          printf '%s\n' "${rpp_client_digest}" > "${digest_path}"
          return 0
        fi
      fi
    done

    >&2 echo "rpp-get-install failed, retrying in 5 seconds"
    sleep 5
  done

  rm -f "${tmp_path}"
  return 1
}
{{- end }}


{{- define "bb-rp" -}}
bb-rp-fire-events() {
  local result_path="$1"
  local action=""
  local package_name=""

  while read -r action package_name; do
    case "${action}" in
      installed)
        bb-event-fire "bb-package-installed" "${package_name}"
        ;;
      removed)
        bb-event-fire "bb-package-removed" "${package_name}"
        ;;
    esac
  done < "${result_path}"
}

# bb-package-install package:digest
bb-package-install() {
  bb-log-deprecated "rpp-get install"

  if [[ "$#" -eq 0 ]]; then
    return 0
  fi

  local result_path="$(mktemp)"

  local rc=0
  rpp-get install --result "${result_path}" "$@" || rc=$?
  bb-rp-fire-events "${result_path}"
  rm -f "${result_path}"

  return "${rc}"
}

# Unpack package from module image and run install script
# bb-package-module-install package:digest repository module_name
bb-package-module-install() {
  bb-log-deprecated "rpp-get install"

  local module_package="$1"

  bb-package-install "${module_package}"
}

# Fetch packages by digest
# bb-package-fetch package1:digest1 [package2:digest2 ...]
bb-package-fetch() {
  bb-log-deprecated "rpp-get fetch"
  rpp-get fetch "$@"
}

# run uninstall script from hold dir
# bb-package-remove package
bb-package-remove() {
  bb-log-deprecated "rpp-get uninstall"

  if [[ "$#" -eq 0 ]]; then
    return 0
  fi

  local result_path="$(mktemp)"

  local rc=0
  rpp-get uninstall --result "${result_path}" "$@" || rc=$?
  bb-rp-fire-events "${result_path}"
  rm -f "${result_path}"

  return "${rc}"
}
{{- end }}




{{- define "get-phase2" -}}
function fetch_bootstrap() {
  local url="$1" token="$2" out="$3"

  local code
  code=$(/opt/deckhouse/bin/d8-curl -sSx "" \
    --connect-timeout 10 \
    "$url" \
    -H "Authorization: Bearer $token" \
    --cacert /var/lib/bashible/ca.crt \
    -o "$out" -w '%{http_code}') || {
      >&2 echo "Error fetching bootstrap from ${url}"
      return 3
    }

  case "$code" in
    200)
      jq -er '.bootstrap' "$out"
      ;;
    401)
      >&2 echo "Bootstrap-token expired."
      return 2
      ;;
    *)
      >&2 echo "HTTP $code: $(head -c 255 "$out" 2>/dev/null)"
      return 1
      ;;
  esac
}

function get_phase2() {
  local bootstrap_ng_name="{{ .nodeGroup.name }}"
  local token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  local out="${TMPDIR}/phase2-response.json"

  local http_401_count=0
  local max_http_401_count=6
  local rc=0

  while true; do
    for server in {{ .Values.nodeManager.internal.clusterMasterAddresses | join " " }}; do
      if fetch_bootstrap \
        "https://${server}/apis/bashible.deckhouse.io/v1alpha1/bootstrap/${bootstrap_ng_name}" \
        "$token" "$out"; then
        rm -f "$out"
        return 0
      else
        rc=$?
      fi

      rm -f "$out"

      if [ "$rc" -eq 2 ]; then
        ((http_401_count++))
        if [ "$http_401_count" -ge "$max_http_401_count" ]; then
          return 1
        fi
      else
        >&2 echo "failed to get bootstrap ${bootstrap_ng_name} from https://${server}/apis/bashible.deckhouse.io/v1alpha1/bootstrap/${bootstrap_ng_name} (exit code $rc)"
      fi
    done

    sleep 10
  done
}
{{- end }}

{{- define "bb-rpp-endpoints" -}}
{{- $clusterMasterKubeAPIEndpoints := list -}}
{{- range $endpoint := .normal.clusterMasterEndpoints -}}
  {{- $clusterMasterKubeAPIEndpoints = append $clusterMasterKubeAPIEndpoints (printf "%s:%v" $endpoint.address $endpoint.kubeApiPort) -}}
{{- end -}}
function get_pods() {
  local namespace=$1
  local labelSelector=$2
  local token=$3

  while true; do
    for server in {{ $clusterMasterKubeAPIEndpoints | join " " }}; do
      url="https://$server/api/v1/namespaces/$namespace/pods?labelSelector=$labelSelector"
      if d8-curl -sS -f -x "" --connect-timeout 10 -X GET "$url" --header "Authorization: Bearer $token" --cacert "$BOOTSTRAP_DIR/ca.crt"
      then
      return 0
      else
        >&2 echo "failed to get $resource $name with curl https://$server..."
      fi
    done
    sleep 10
  done
}

function get_rpp_address() {
  if [ -f /var/lib/bashible/bootstrap-token ]; then
    local token="$(</var/lib/bashible/bootstrap-token)"
    local namespace="d8-cloud-instance-manager"
    local labelSelector="app%3Dregistry-packages-proxy"

    rpp_ips=$(get_pods $namespace $labelSelector $token | jq -r '.items[] | select(.status.phase == "Running") | .status.podIP')
    port=4300
    ips_csv=$(echo "$rpp_ips" | grep -v '^[[:space:]]*$' | sed "s/$/:$port/" | tr '\n' ',' | sed 's/,$//')
    echo "$ips_csv"
  fi
}
{{- end }}
