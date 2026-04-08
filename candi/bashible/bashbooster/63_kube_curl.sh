# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

bb-curl-kube-detect-auth() {
  if [[ -f /var/lib/kubelet/pki/kubelet-client-current.pem ]] && [[ -f /etc/kubernetes/kubelet.conf ]]; then
    BB_KUBE_AUTH_TYPE="cert"
  elif [[ -f /etc/kubernetes/admin.conf ]]; then
    BB_KUBE_AUTH_TYPE="admin-cert"
    bb-curl-kube-extract-admin-certs
  elif [[ -f /var/lib/bashible/bootstrap-token ]]; then
    BB_KUBE_AUTH_TYPE="bootstrap-token"
  else
    >&2 echo "bb-curl-kube: cannot detect auth — no kubelet cert, admin.conf, or bootstrap-token found"
    return 1
  fi
  export BB_KUBE_AUTH_TYPE
}

bb-curl-kube-extract-admin-certs() {
  local admin_conf="/etc/kubernetes/admin.conf"
  local cert_file="${TMPDIR}/bb-kube-admin-cert.pem"
  local key_file="${TMPDIR}/bb-kube-admin-key.pem"

  awk '/client-certificate-data:/{print $2}' "$admin_conf" | base64 -d > "$cert_file"
  awk '/client-key-data:/{print $2}' "$admin_conf" | base64 -d > "$key_file"
  chmod 600 "$cert_file" "$key_file"
}

bb-curl-kube-build-curl-auth-args() {
  BB_KUBE_AUTH_ARGS=()
  case "${BB_KUBE_AUTH_TYPE}" in
    cert)
      BB_KUBE_AUTH_ARGS=(
        --cacert /etc/kubernetes/pki/ca.crt
        --cert /var/lib/kubelet/pki/kubelet-client-current.pem
      )
      ;;
    admin-cert)
      BB_KUBE_AUTH_ARGS=(
        --cacert /etc/kubernetes/pki/ca.crt
        --cert "${TMPDIR}/bb-kube-admin-cert.pem"
        --key "${TMPDIR}/bb-kube-admin-key.pem"
      )
      ;;
    bootstrap-token)
      BB_KUBE_AUTH_ARGS=(
        --header "Authorization: Bearer $(</var/lib/bashible/bootstrap-token)"
        --cacert "${BOOTSTRAP_DIR:-/var/lib/bashible}/ca.crt"
      )
      ;;
    *)
      >&2 echo "bb-curl-kube: unknown auth type: ${BB_KUBE_AUTH_TYPE}"
      return 1
      ;;
  esac
}

bb-curl-kube-resolve-endpoint() {
  case "${BB_KUBE_AUTH_TYPE}" in
    cert)
      local kube_server
      kube_server="$(grep -m1 'server:' /etc/kubernetes/kubelet.conf | awk '{print $2}')"
      if [[ -n "$kube_server" ]] && bb-curl-kube-healthz "$kube_server"; then
        BB_KUBE_APISERVER_URL="$kube_server"
      else
        for server in ${BB_KUBE_APISERVER_FALLBACK_ENDPOINTS:-}; do
          if bb-curl-kube-healthz "https://$server"; then
            BB_KUBE_APISERVER_URL="https://$server"
            break
          fi
        done
      fi
      ;;
    admin-cert)
      BB_KUBE_APISERVER_URL="$(grep -m1 'server:' /etc/kubernetes/admin.conf | awk '{print $2}')"
      ;;
    bootstrap-token)
      for server in ${BB_KUBE_APISERVER_FALLBACK_ENDPOINTS:-}; do
        if bb-curl-kube-healthz "https://$server"; then
          BB_KUBE_APISERVER_URL="https://$server"
          break
        fi
      done
      ;;
  esac

  if [[ -z "${BB_KUBE_APISERVER_URL:-}" ]]; then
    >&2 echo "bb-curl-kube: cannot resolve API server endpoint"
    return 1
  fi
  export BB_KUBE_APISERVER_URL
}

bb-curl-kube() {
  local api_path="$1"
  shift

  if [[ -z "${BB_KUBE_AUTH_TYPE:-}" ]]; then
    bb-curl-kube-detect-auth || return 1
  fi

  if [[ -z "${BB_KUBE_APISERVER_URL:-}" ]]; then
    bb-curl-kube-resolve-endpoint || return 1
  fi

  local -a auth_args=()
  bb-curl-kube-build-curl-auth-args
  auth_args=("${BB_KUBE_AUTH_ARGS[@]}")

  d8-curl -sS -f -x "" --connect-timeout 10 --max-time 60 \
    "${auth_args[@]}" \
    "$@" \
    "${BB_KUBE_APISERVER_URL}${api_path}"
}

bb-curl-kube-healthz() {
  local server_url="$1"

  local -a auth_args=()
  if [[ -n "${BB_KUBE_AUTH_TYPE:-}" ]]; then
    bb-curl-kube-build-curl-auth-args
    auth_args=("${BB_KUBE_AUTH_ARGS[@]}")
  fi

  d8-curl -sS -f -x "" --connect-timeout 3 --max-time 3 \
    "${auth_args[@]}" \
    "${server_url}/healthz" >/dev/null 2>&1
}

bb-curl-kube-patch-node-metadata() {
  local node_name="$1"
  local field="$2"
  shift 2

  local resource_version=""
  if [[ "${1:-}" == --resource-version=* ]]; then
    resource_version="${1#--resource-version=}"
    shift
  fi

  local json_obj="{}"
  for arg in "$@"; do
    if [[ "$arg" == *=* ]]; then
      local key="${arg%%=*}"
      local value="${arg#*=}"
      json_obj=$(jq --arg k "$key" --arg v "$value" '.[$k] = $v' <<< "$json_obj")
    else
      local key="${arg%-}"
      json_obj=$(jq --arg k "$key" '.[$k] = null' <<< "$json_obj")
    fi
  done

  local patch
  if [[ -n "$resource_version" ]]; then
    patch=$(jq -nc --arg rv "$resource_version" --arg f "$field" --argjson obj "$json_obj" \
      '{"metadata":{"resourceVersion":$rv,($f):$obj}}')
  else
    patch=$(jq -nc --arg f "$field" --argjson obj "$json_obj" \
      '{"metadata":{($f):$obj}}')
  fi

  bb-curl-kube "/api/v1/nodes/${node_name}" \
    -X PATCH \
    -H "Content-Type: application/strategic-merge-patch+json" \
    --data "$patch"
}


