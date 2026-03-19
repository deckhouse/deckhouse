
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

  if [[ -f "${minget_path}" ]]; then
    return 0
  fi

  MINGET_B64=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
  mkdir -p /opt/deckhouse/bin/
  echo $MINGET_B64 | base64 -d > "${minget_path}"
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
