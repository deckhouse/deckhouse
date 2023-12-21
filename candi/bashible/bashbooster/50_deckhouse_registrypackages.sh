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

# shellcheck disable=SC2211,SC2153

bb-var BB_RP_INSTALLED_PACKAGES_STORE "/var/cache/registrypackages"
bb-var BB_RP_FETCHED_PACKAGES_STORE "${TMPDIR}/registrypackages"

# Use d8-curl if installed, fallback to system package if not
bb-rp-curl() {
  if command -v d8-curl > /dev/null ; then
    d8-curl -4 --remove-on-error --parallel "$@"
  else
    curl "$@"
  fi
}

# check if package installed
# bb-rp-is-installed? package digest
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

# Check if package fetched
# bb-rp-is-fetched? package digest
bb-rp-is-fetched?() {
  if [[ -d "${BB_RP_FETCHED_PACKAGES_STORE}/${1}" ]]; then
    if [[ -f "${BB_RP_FETCHED_PACKAGES_STORE}/${1}/${2}.tar.gz" ]]; then
      return 0
    fi
  fi
  return 1
}

# get token from registry auth
# bb-rp-get-token
 bb-rp-get-token() {
  local AUTH=""
  local AUTH_HEADER=""
  local AUTH_REALM=""
  local AUTH_SERVICE=""

  if [[ -n ${REGISTRY_AUTH} ]]; then
    AUTH="-u ${REGISTRY_AUTH}"
  fi

  AUTH_HEADER="$(bb-rp-curl --retry 3 -sSLi "${SCHEME}://${REGISTRY_ADDRESS}/v2/" | grep -i "www-authenticate")"
  AUTH_REALM="$(grep -oE 'Bearer realm="http[s]{0,1}://[a-z0-9\.\:\/\-]+"' <<< ${AUTH_HEADER} | cut -d '"' -f2)"
  AUTH_SERVICE="$(grep -oE 'service="[[:print:]]+"' <<< "${AUTH_HEADER}" | cut -d '"' -f2 | sed 's/ /+/g')"
  if [ -z ${AUTH_REALM} ]; then
    bb-exit 1 "couldn't find bearer realm parameter, consider enabling bearer token auth in your registry, returned header: ${AUTH_HEADER}"
  fi
  # shellcheck disable=SC2086
  # Remove leading / from REGISTRY_PATH due to scope format -> scope=repository:deckhouse/fe:pull
  bb-rp-curl --retry 3 -fsSL ${AUTH} "${AUTH_REALM}?service=${AUTH_SERVICE}&scope=repository:${REGISTRY_PATH#/}:pull" | jq -r '.token'
}

# Fetch manifests from registry and save under $BB_RP_FETCHED_PACKAGES_STORE
# bb-rp-fetch-manifests map[digest]package_name
#
# This function uses the PACKAGES_MAP variable from the scope of the bb-rp-fetch()
# due to the limitations of using `declare -n` in CentOS 7 (bash 4.2, and 4.3 is needed).
# DO NOT CALL THIS FUNCTION DIRECTLY!
bb-rp-fetch-manifests() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token)"

  local URLs=()
  # key - digest to fetch, value - package name
  local PACKAGE_DIGEST
  for PACKAGE_DIGEST in "${!PACKAGES_MAP[@]}"; do
    local PACKAGE_DIR="${BB_RP_FETCHED_PACKAGES_STORE}/${PACKAGES_MAP[$PACKAGE_DIGEST]}"
    URLs+=(
      -o "${PACKAGE_DIR}/manifest.json"
      "${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/manifests/${PACKAGE_DIGEST}"
    )
  done

  bb-rp-curl --retry 3 -fsSL --create-dirs \
    -H "Authorization: Bearer ${TOKEN}" \
    -H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
    "${URLs[@]}"
}

# Fetch digests from registry and save to file
# bb-rp-fetch-blobs map[blob_digest]output_file_path
#
# This function uses the BLOB_FILES_MAP variable from the scope of the bb-rp-fetch()
# due to the limitations of using `declare -n` in CentOS 7 (bash 4.2, and 4.3 is needed).
# DO NOT CALL THIS FUNCTION DIRECTLY!
bb-rp-fetch-blobs() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token)"

  local URLs=()
  # key - digest to fetch, value - output file path
  local BLOB_DIGEST
  for BLOB_DIGEST in "${!BLOB_FILES_MAP[@]}"; do
    URLs+=(
      -o "${BLOB_FILES_MAP[$BLOB_DIGEST]}"
      "${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/blobs/${BLOB_DIGEST}"
    )
  done

  bb-rp-curl --retry 3 -fsSLH "Authorization: Bearer ${TOKEN}" "${URLs[@]}"
}

# Fetch packages by digest
# bb-rp-fetch package1:digest1 [package2:digest2 ...]
bb-rp-fetch() {
  mkdir -p "${BB_RP_FETCHED_PACKAGES_STORE}"

  declare -A PACKAGES_MAP
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    if bb-rp-is-installed? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package already installed"
      continue
    fi

    if bb-rp-is-fetched? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package already fetched"
      continue
    fi

    PACKAGES_MAP[$DIGEST]="${PACKAGE}"
  done

  if [ "${#PACKAGES_MAP[@]}" -eq 0 ]; then
    return 0
  fi

  bb-log-info "Fetching manifests: ${PACKAGES_MAP[*]}"
  bb-rp-fetch-manifests PACKAGES_MAP
  if bb-error?; then
    bb-log-error "Failed to fetch manifests"
    return $BB_ERROR
  fi

  declare -A BLOB_FILES_MAP
  local PACKAGE_DIGEST
  for PACKAGE_DIGEST in "${!PACKAGES_MAP[@]}"; do
    local PACKAGE_DIR="${BB_RP_FETCHED_PACKAGES_STORE}/${PACKAGES_MAP[$PACKAGE_DIGEST]}"
    jq -er '.layers[-1].digest' "${PACKAGE_DIR}/manifest.json" > "${PACKAGE_DIR}/top_layer_digest"
    BLOB_FILES_MAP[$(cat "${PACKAGE_DIR}/top_layer_digest")]="${PACKAGE_DIR}/${PACKAGE_DIGEST}.tar.gz"
  done

  bb-log-info "Fetching packages: ${PACKAGES_MAP[*]}"
  bb-rp-fetch-blobs BLOB_FILES_MAP
  if bb-error?; then
    bb-log-error "Failed to fetch packages"
    return "${BB_ERROR}"
  fi
  bb-log-info "Packages saved under ${BB_RP_FETCHED_PACKAGES_STORE}"
}


# Unpack packages and run install script
# bb-rp-install package:digest
bb-rp-install() {
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    if bb-rp-is-installed? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package already installed"
      continue
    fi

    if ! bb-rp-is-fetched? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package not found locally"
      bb-rp-fetch "${PACKAGE_WITH_DIGEST}"
    fi

    bb-log-info "Unpacking package '${PACKAGE}'"
    local TMP_DIR=""
    TMP_DIR="$(mktemp -d)"
    tar -xf "${BB_RP_FETCHED_PACKAGES_STORE}/${PACKAGE}/${DIGEST}.tar.gz" -C "${TMP_DIR}"
    if bb-error?
    then
        rm -rf "${TMP_DIR}" "${BB_RP_FETCHED_PACKAGES_STORE:?}/${PACKAGE}"
        bb-log-error "Failed to unpack package '${PACKAGE}', it may be corrupted. The package will be refetched on the next attempt"
        return $BB_ERROR
    fi

    bb-log-info "Installing package '${PACKAGE}'"
    # shellcheck disable=SC2164
    pushd "${TMP_DIR}" >/dev/null
    ./install
    if bb-error?
    then
        popd >/dev/null
        rm -rf "${TMP_DIR}"
        bb-log-error "Failed to install package '${PACKAGE}'"
        return $BB_ERROR
    fi
    popd >/dev/null

    # Write digest to hold file
    mkdir -p "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    echo "${DIGEST}" > "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}/digest"
    # Copy install/uninstall scripts to hold dir
    cp "${TMP_DIR}/install" "${TMP_DIR}/uninstall" "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    # Cleanup
    rm -rf "${TMP_DIR}" "${BB_RP_FETCHED_PACKAGES_STORE:?}/${PACKAGE}"

    bb-log-info "'${PACKAGE}' package successfully installed"
    bb-event-fire "bb-package-installed" "${PACKAGE}"
    trap - ERR
  done
}

# run uninstall script from hold dir
# bb-rp-remove package
bb-rp-remove() {
  for PACKAGE in "$@"; do
    if [[ -f "${BB_RP_INSTALLED_PACKAGES_STORE:?}/${PACKAGE:?}/uninstall" ]]; then
      bb-log-info "Removing package '${PACKAGE}'"
      # shellcheck disable=SC1090
      . "${BB_RP_INSTALLED_PACKAGES_STORE:?}/${PACKAGE:?}/uninstall"
      bb-exit-on-error "Failed to remove package '${PACKAGE}'"
      # cleanup
      rm -rf "${BB_RP_INSTALLED_PACKAGES_STORE:?}/${PACKAGE:?}"
      bb-event-fire "bb-package-removed" "${PACKAGE}"
    fi
  done
}
