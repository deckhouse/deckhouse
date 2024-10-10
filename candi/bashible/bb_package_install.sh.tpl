{{- /*
# Copyright 2024 Flant JSC
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
set -Eeo pipefail

export PATH="/opt/deckhouse/bin:$PATH"
export LANG=C
export BB_INSTALLED_PACKAGES_STORE="/var/cache/registrypackages"
export BB_FETCHED_PACKAGES_STORE="/${TMPDIR}/registrypackages"
export TMPDIR="/opt/deckhouse/tmp"
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

{{- if $check_python := .Files.Get "/deckhouse/candi/bashible/check_python.sh.tpl" | default (.Files.Get "candi/bashible/check_python.sh.tpl") -}}
  {{- tpl ( $check_python ) . | nindent 0 }}
{{- end }}

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

    retries=0
    while [ "$retries" -lt 30 ]
    do
      retries=$(( retries+1 ))
      bb-package-fetch-blob "${PACKAGE_DIGEST}" "${PACKAGE_DIR}/${PACKAGE_DIGEST}.tar.gz" && break
			sleep 5
    done
  done
}

bb-package-fetch-blob() {
  local REPOSITORY="${REPOSITORY:-}"
  local REPOSITORY_PATH="${REPOSITORY_PATH:-}"

  check_python

  cat - <<EOFILE | $python_binary
import random
import ssl
try:
  from urllib.request import urlopen, Request, HTTPError
except ImportError:
  from urllib2 import urlopen, Request, HTTPError
endpoints = "${PACKAGES_PROXY_ADDRESSES}".split(",")
# Choose a random endpoint to increase fault tolerance and reduce load on a single endpoint.
random.shuffle(endpoints)
ssl._create_default_https_context = ssl._create_unverified_context
for ep in endpoints:
  url = 'https://{}/package?digest=$1&repository=${REPOSITORY}&path=${REPOSITORY_PATH}'.format(ep)
  request = Request(url, headers={'Authorization': 'Bearer ${PACKAGES_PROXY_TOKEN}'})
  try:
    response = urlopen(request, timeout=300)
  except HTTPError as e:
    print("Access to {} return HTTP Error {}: {}".format(url, e.getcode(), e.read()[:255]))
    print('You can check via curl -v -k -H "Authorization: Bearer ${PACKAGES_PROXY_TOKEN}" "{}" > /dev/null'.format(url))
    continue
  except Exception as e:
    print("Access to {} return Error: {}".format(url, e))
    continue
  break
with open('$2', 'wb') as f:
    f.write(response.read())
EOFILE
}
