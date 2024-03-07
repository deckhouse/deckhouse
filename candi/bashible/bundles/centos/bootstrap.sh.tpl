{{- /*
# Copyright 2021 Flant JSC
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
*/}}
#!/bin/bash

{{- /*
# Bashible framework uses jq for many operations, so jq must be installed in the first step of node bootstrap.
# But there is no package jq in centos/redhat (the jq package is located in the epel repository).
# We need to install jq from packages registry. But library for install packages requires jq.
# To avoid this problem we use modified version of registry package helper functions, with python instead of jq.
# When we will move to Centos 8, we should install jq from main repo.

# If package registry uses self-signed certificate, we need to pass ca certificate to curl to verify certificate passed
# from registry. But we cannot pass ca certificate to bootstrap script due to to size limitation of cloud-init scripts in some cloud providers (AWS<=16kb).
# Instead we pass -k (insecure) flag to curl as workaround solution.

# Now we render bootstrap with helm and parse registry.path and registry.dockerCfg to get registry host and auth credentials by unclear way.
# Later, we plan render bootstrap with bashible-apiserver and use registry.host and registry.auth variables.
# https://github.com/deckhouse/deckhouse/issues/143
*/}}
{{- /*
# By default, python is not installed on CentOS 8.
# So we need to install it before first use
*/}}
. /etc/os-release
if [ "${VERSION_ID}" == "8" ] ; then
  yum install python3 -y
  alternatives --set python /usr/bin/python3
fi
{{- /*
Description of problem with XFS https://www.suse.com/support/kb/doc/?id=000020068
*/}}
for FS_NAME in $(mount -l -t xfs | awk '{ print $1 }'); do
  if command -v xfs_info >/dev/null && xfs_info $FS_NAME | grep -q ftype=0; then
     >&2 echo "ERROR: XFS file system with ftype=0 was found ($FS_NAME)."
     exit 1
  fi
done

REGISTRY_ADDRESS="{{ .registry.address }}"
SCHEME="{{ .registry.scheme }}"
REGISTRY_PATH="{{ .registry.path }}"
{{- if hasKey .registry "auth" }}
REGISTRY_AUTH="$(base64 -d <<< "{{ .registry.auth | default "" }}")"
{{- else }}
REGISTRY_AUTH="$(base64 -d <<< "{{ .registry.dockerCfg }}" | python -c 'import json; import sys; dockerCfg = sys.stdin.read(); parsed = json.loads(dockerCfg); parsed["auths"]["'${REGISTRY_ADDRESS}'"].setdefault("auth", ""); print(parsed["auths"]["'${REGISTRY_ADDRESS}'"]["auth"]);' | base64 -d)"
{{- end }}
BB_RP_INSTALLED_PACKAGES_STORE="/var/cache/registrypackages"
{{- /*
# check if image installed
# bb-rp-is-installed? package digest
*/}}
bb-rp-is-installed?() {
  if [[ -d "${BB_RP_INSTALLED_PACKAGES_STORE}/${1}" ]]; then
    local INSTALLED_DIGEST=""
    INSTALLED_DIGEST="$(cat "${BB_RP_INSTALLED_PACKAGES_STORE}/${1}/digest")"
    if [[ "${INSTALLED_DIGEST}" == "${2}" ]]; then
      return 0
    fi
  fi
  return 1
}
{{- /*
# get token from registry auth
# bb-rp-get-token
*/}}
bb-rp-get-token() {
  local AUTH=""
  local AUTH_HEADER=""
  local AUTH_REALM=""
  local AUTH_SERVICE=""

  if [[ -n "${REGISTRY_AUTH}" ]]; then
    AUTH="yes"
  fi

  AUTH_HEADER="$(curl --connect-timeout 10 --max-time 300 --retry 3 -k -sSLi "${SCHEME}://${REGISTRY_ADDRESS}/v2/" | grep -i "www-authenticate")"
  AUTH_REALM="$(grep -oE 'Bearer realm="http[s]{0,1}://[a-z0-9\.\:\/\-]+"' <<< "${AUTH_HEADER}" | cut -d '"' -f2)"
  AUTH_SERVICE="$(grep -oE 'service="[[:print:]]+"' <<< "${AUTH_HEADER}" | cut -d '"' -f2 | sed 's/ /+/g')"
  if [ -z "${AUTH_REALM}" ]; then
    >&2 echo "couldn't find bearer realm parameter, consider enabling bearer token auth in your registry, returned header: ${AUTH_HEADER}"
    return 1
  fi
{{- /*
  # Remove leading / from REGISTRY_PATH due to scope format -> scope=repository:deckhouse/fe:pull
*/}}
  curl --connect-timeout 10 --max-time 300 --retry 3 -k -fsSL ${AUTH:+-u "$REGISTRY_AUTH"} "${AUTH_REALM}?service=${AUTH_SERVICE}&scope=repository:${REGISTRY_PATH#/}:pull" | python -c 'import json; import sys; jsonDoc = sys.stdin.read(); parsed = json.loads(jsonDoc); print(parsed["token"]);'
}
{{- /*
# fetch manifest from registry and get list of digests
# bb-rp-get-digests digest
*/}}
bb-rp-get-digests() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token)"
  curl --connect-timeout 10 --max-time 300 --retry 3 -k -fsSL \
			-H "Authorization: Bearer ${TOKEN}" \
			-H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
        "${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/manifests/${1}" | python -c 'import json; import sys; jsonDoc = sys.stdin.read(); parsed = json.loads(jsonDoc); print(parsed["layers"][-1]["digest"])'
}
{{- /*
# Fetch digest from registry
# bb-rp-fetch-digest digest outfile
*/}}
bb-rp-fetch-digest() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token)"
  curl --connect-timeout 10 --max-time 300 --retry 3 -k -sSLH "Authorization: Bearer ${TOKEN}" "${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/blobs/${1}" -o "${2}"
}
{{- /*
# download package digests, unpack them and run install script
# bb-rp-install package:digest
*/}}
bb-rp-install() {
  shopt -u failglob

  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    if bb-rp-is-installed? "${PACKAGE}" "${DIGEST}"; then
      continue
    fi

    local DIGESTS=""
    DIGESTS="$(bb-rp-get-digests "${DIGEST}")"

    local TMPDIR=""
    TMPDIR="$(mktemp -d)"

    for TMPDIGEST in ${DIGESTS}; do
      local TMPFILE=""
      TMPFILE="$(mktemp -u)"
      bb-rp-fetch-digest "${TMPDIGEST}" "${TMPFILE}"
      tar -xf "${TMPFILE}" -C "${TMPDIR}"
      rm -f "${TMPFILE}"
    done

    pushd "${TMPDIR}" >/dev/null
    ./install
    # shellcheck disable=SC2164
    popd >/dev/null

    mkdir -p "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    echo "${DIGEST}" > "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}/digest"
    cp "${TMPDIR}/install" "${TMPDIR}/uninstall" "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    rm -rf "${TMPDIR}"
  done

  shopt -s failglob
}
export PATH="/opt/deckhouse/bin:$PATH"
export LANG=C
{{- if .proxy }}
  {{- if .proxy.httpProxy }}
export HTTP_PROXY={{ .proxy.httpProxy | quote }}
export http_proxy=${HTTP_PROXY}
  {{- end }}
  {{- if .proxy.httpsProxy }}
export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
export https_proxy=${HTTPS_PROXY}
  {{- end }}
  {{- if .proxy.noProxy }}
export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
export no_proxy=${NO_PROXY}
  {{- end }}
{{- else }}
  unset HTTP_PROXY http_proxy HTTPS_PROXY https_proxy NO_PROXY no_proxy
{{- end }}
yum updateinfo
until yum install nc curl wget -y; do
  echo "Error installing packages"
  yum updateinfo
  sleep 10
done
{{- /*
# Install jq from deckhouse registry.
# When we will move to Centos 8, we should install jq from main repo.
*/}}
yum install jq -y || bb-package-install "jq:{{ .images.registrypackages.jq16 }}"
mkdir -p /var/lib/bashible/
