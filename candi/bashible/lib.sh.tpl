
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
{{- $images := .images | default (dict) -}}
{{- $registryPackages := get $images "registrypackages" | default (dict) -}}
{{- if and (ne .runType "Normal") .mingetB64 }}
bb-minget-install() {
  local path="/opt/deckhouse/bin/minget"

  if [[ -s "$path" && -x "$path" ]]; then
    return 0
  fi

  mkdir -p "${path%/*}"
  if ! echo -n '{{ .mingetB64 }}' | base64 -d > "$path"; then
    rm -f "$path"
    return 1
  fi
  if [[ ! -s "$path" ]]; then
    rm -f "$path"
    return 1
  fi
  chmod +x "$path"
}
{{- end }}

bb-rpp-get-binary-ready() {
  local version

  version="$("$1" version 2>/dev/null)" && [[ -n $version ]]
}

bb-rpp-get-fetch() {
  if command -v d8-curl >/dev/null 2>&1; then
    d8-curl -sS -f -x "" --connect-timeout 10 --max-time 300 "http://$1"
    return
  fi

  /opt/deckhouse/bin/minget "$1"
}

bb-rpp-get-install() {
  local bin="/opt/deckhouse/bin/rpp-get"
  local digest="{{ get $registryPackages "rppGet" }}"
  local digest_file="${BB_RP_INSTALLED_PACKAGES_STORE:-/var/cache/registrypackages}/rpp-get/digest"
  local tmp="${bin}.tmp"
  local prefix="${PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID:+/${PACKAGES_PROXY_BOOTSTRAP_CLUSTER_UUID}}"
  local max_attempts=30 attempt address

  if [[ -f "$digest_file" &&
        "$(<"$digest_file")" == "$digest" ]] &&
     bb-rpp-get-binary-ready "$bin"; then
    return 0
  fi

  if [[ -z "${PACKAGES_PROXY_BOOTSTRAP_ADDRESSES:-}" ]]; then
    >&2 echo "rpp-get bootstrap source is not configured"
    return 1
  fi

  mkdir -p "${bin%/*}" "${digest_file%/*}"

  for ((attempt = 1; attempt <= max_attempts; attempt++)); do
    for address in ${PACKAGES_PROXY_BOOTSTRAP_ADDRESSES}; do
      bb-rpp-get-fetch "${address}${prefix}/rpp-get?digest=${digest}" > "$tmp" || continue
      chmod +x "$tmp"
      bb-rpp-get-binary-ready "$tmp" || continue

      mv -f "$tmp" "$bin"
      echo "$digest" > "$digest_file"
      return 0
    done

    >&2 echo "rpp-get-install failed (${attempt}/${max_attempts}), retrying in 5 seconds"
    sleep 5
  done

  >&2 echo "rpp-get-install failed after ${max_attempts} attempts"
  rm -f "$tmp"
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
fetch_bootstrap() {
  local url="$1" token="$2" out="$3" code

  code=$(/opt/deckhouse/bin/d8-curl -sSx "" \
    --connect-timeout 10 \
    "$url" \
    -H "Authorization: Bearer $token" \
    --cacert "$BOOTSTRAP_DIR/ca.crt" \
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

get_phase2() {
  local token="$(<${BOOTSTRAP_DIR}/bootstrap-token)"
  local out="${TMPDIR}/phase2-response.json"
  local path="/apis/bashible.deckhouse.io/v1alpha1/bootstrap/{{ .nodeGroup.name }}"
  local count_401=0
  local rc server url

  while :; do
    for server in {{ .Values.nodeManager.internal.clusterMasterAddresses | join " " }}; do
      url="https://${server}${path}"
      if fetch_bootstrap "$url" "$token" "$out"; then
        rm -f "$out"
        return 0
      else
        rc=$?
      fi

      rm -f "$out"

      if (( rc == 2 )); then
        ((count_401++))
        if (( count_401 >= 6 )); then
          return 1
        fi
      else
        >&2 echo "failed to get bootstrap from ${url} (exit code $rc)"
      fi
    done

    sleep 10
  done
}
{{- end }}

{{- define "bb-rpp-endpoints" -}}
function get_pods() {
  local namespace=$1
  local labelSelector=$2
  local token=$3

  while true; do
    for server in {{ .clusterMasterKubeAPIEndpoints | join " " }}; do
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
