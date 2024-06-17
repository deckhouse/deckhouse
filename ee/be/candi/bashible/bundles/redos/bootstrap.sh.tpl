{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.
*/}}
#!/bin/bash

function check_python() {
  for pybin in python3 python2 python; do
    if command -v "$pybin" >/dev/null 2>&1; then
      python_binary="$pybin"
      return 0
    fi
  done
  echo "Python not found, exiting..."
  return 1
}

bb-package-install() {
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"
    bb-package-fetch "${PACKAGE_WITH_DIGEST}"
    local TMP_DIR=""
    TMP_DIR="$(mktemp -d)"
    tar -xf "${BB_FETCHED_PACKAGES_STORE}/${PACKAGE}/${DIGEST}.tar.gz" -C "${TMP_DIR}"

    # shellcheck disable=SC2164
    pushd "${TMP_DIR}" >/dev/null
    ./install
    popd >/dev/null
    mkdir -p "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    echo "${DIGEST}" > "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}/digest"
    cp "${TMP_DIR}/install" "${TMP_DIR}/uninstall" "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    rm -rf "${TMP_DIR}" "${BB_FETCHED_PACKAGES_STORE:?}/${PACKAGE}"
  done
}

bb-package-fetch() {
  mkdir -p "${BB_FETCHED_PACKAGES_STORE}"
  declare -A PACKAGES_MAP
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"
    PACKAGES_MAP[$DIGEST]="${PACKAGE}"
  done
  bb-package-fetch-blobs PACKAGES_MAP
}

bb-package-fetch-blobs() {
  local PACKAGE_DIGEST
  for PACKAGE_DIGEST in "${!PACKAGES_MAP[@]}"; do
    local PACKAGE_DIR="${BB_FETCHED_PACKAGES_STORE}/${PACKAGES_MAP[$PACKAGE_DIGEST]}"
    mkdir -p "${PACKAGE_DIR}"
    bb-package-fetch-blob "${PACKAGE_DIGEST}" "${PACKAGE_DIR}/${PACKAGE_DIGEST}.tar.gz"
  done
}
bb-package-fetch-blob() {
  check_python

  cat - <<EOF | $python_binary
import random
import ssl
try:
    from urllib.request import urlopen, Request
except ImportError as e:
    from urllib2 import urlopen, Request
# Choose a random endpoint to increase fault tolerance and reduce load on a single endpoint.
endpoints = "${PACKAGES_PROXY_ADDRESSES}".split(",")
endpoint = random.choice(endpoints)
ssl._create_default_https_context = ssl._create_unverified_context
url = 'https://{}/package?digest=$1&repository=${REPOSITORY}'.format(endpoint)
request = Request(url, headers={'Authorization': 'Bearer ${PACKAGES_PROXY_TOKEN}'})
response = urlopen(request, timeout=300)
with open('$2', 'wb') as f:
    f.write(response.read())
EOF
}

export PATH="/opt/deckhouse/bin:$PATH"
export LANG=C
export REPOSITORY=""
export BB_INSTALLED_PACKAGES_STORE="/var/cache/registrypackages"
export BB_FETCHED_PACKAGES_STORE="/${TMPDIR}/registrypackages"
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
{{- if .packagesProxy }}
export PACKAGES_PROXY_ADDRESSES="{{ .packagesProxy.addresses | join "," }}"
export PACKAGES_PROXY_TOKEN="{{ .packagesProxy.token }}"
{{- end }}
bb-package-install "jq:{{ .images.registrypackages.jq16 }}" "curl:{{ .images.registrypackages.d8Curl821 }}" "netcat:{{ .images.registrypackages.netcat110481 }}"

{{- /*
Description of problem with XFS https://www.suse.com/support/kb/doc/?id=000020068
*/}}
for FS_NAME in $(mount -l -t xfs | awk '{ print $1 }'); do
  if command -v xfs_info >/dev/null && xfs_info $FS_NAME | grep -q ftype=0; then
     >&2 echo "ERROR: XFS file system with ftype=0 was found ($FS_NAME)."
     exit 1
  fi
done

mkdir -p /var/lib/bashible/
