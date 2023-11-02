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

bb-var BB_RP_INSTALLED_PACKAGES_STORE "/var/cache/registrypackages"
bb-var BB_RP_FETCHED_PACKAGES_STORE "${TMPDIR}/registrypackages"
# shellcheck disable=SC2153

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
# bb-rp-is-fetched? digest
bb-rp-is-fetched?() {
  if [[ -d "${BB_RP_FETCHED_PACKAGES_STORE}" ]]; then
    if [[ -f "${BB_RP_FETCHED_PACKAGES_STORE}/${1}" ]]; then
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

  AUTH_HEADER="$(curl --retry 3 -sSLi "${SCHEME}://${REGISTRY_ADDRESS}/v2/" | grep -i "www-authenticate")"
  AUTH_REALM="$(grep -oE 'Bearer realm="http[s]{0,1}://[a-z0-9\.\:\/\-]+"' <<< ${AUTH_HEADER} | cut -d '"' -f2)"
  AUTH_SERVICE="$(grep -oE 'service="[[:print:]]+"' <<< "${AUTH_HEADER}" | cut -d '"' -f2 | sed 's/ /+/g')"
  if [ -z ${AUTH_REALM} ]; then
    bb-exit 1 "couldn't find bearer realm parameter, consider enabling bearer token auth in your registry, returned header: ${AUTH_HEADER}"
  fi
  # shellcheck disable=SC2086
  # Remove leading / from REGISTRY_PATH due to scope format -> scope=repository:deckhouse/fe:pull
  curl --retry 3 -fsSL ${AUTH} "${AUTH_REALM}?service=${AUTH_SERVICE}&scope=repository:${REGISTRY_PATH#/}:pull" | jq -r '.token'
}

# fetch manifest from registry and get list of digests
# bb-rp-get-digests digest
bb-rp-get-digests() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token)"
  curl --retry 3 -fsSL \
			-H "Authorization: Bearer ${TOKEN}" \
			-H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
			"${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/manifests/${1}" | jq -r '.layers[-1].digest'
}

# Fetch digest from registry
# bb-rp-fetch-digest digest outfile
bb-rp-fetch-digest() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token)"
  curl --retry 3 -fsSLH "Authorization: Bearer ${TOKEN}" "${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/blobs/${1}" -o "${2}"
}


# Fetch packages by digest
# bb-rp-fetch package1:digest1 [package2:digest2]
bb-rp-fetch() {
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    # shellcheck disable=SC2211
    if bb-rp-is-installed? "${PACKAGE}" "${DIGEST}"; then
      continue
    fi

    # shellcheck disable=SC2211
    if bb-rp-is-fetched? "${DIGEST}"; then
      bb-log-info "'${PACKAGE}:${DIGEST}' package already fetched"
      continue
    fi

    local TOP_LAYER_DIGEST=""
    TOP_LAYER_DIGEST="$(bb-rp-get-digests "${DIGEST}")"

    # if bb-error?; then
    #   bb-log-error "Failed to get top layer digest for '${PACKAGE}:${DIGEST}'"
    #   return $BB_ERROR
    # fi

    # Get blobs
    mkdir -p "${BB_RP_FETCHED_PACKAGES_STORE}"

    bb-log-info "Fetching package '${PACKAGE}'"
    bb-rp-fetch-digest "${TOP_LAYER_DIGEST}" "${BB_RP_FETCHED_PACKAGES_STORE}/${DIGEST}"
    # shellcheck disable=SC2211
    if bb-error?; then
      bb-log-error "Failed to fetch package '${PACKAGE}'"
      return "${BB_ERROR}"
    fi
    bb-log-info "'${PACKAGE}' package saved to '${BB_RP_FETCHED_PACKAGES_STORE}/${DIGEST}'"
  done
}


# Unpack packages and run install script
# bb-rp-install package:digest
bb-rp-install() {
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    # shellcheck disable=SC2211
    if bb-rp-is-installed? "${PACKAGE}" "${DIGEST}"; then
      continue
    fi

    # shellcheck disable=SC2211
    if ! bb-rp-is-fetched? "${DIGEST}"; then
      bb-log-info "'${PACKAGE}:${DIGEST}' package not found locally"
      bb-rp-fetch "${PACKAGE_WITH_DIGEST}"
      continue
    fi

    trap "rm -rf ${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}" ERR

    bb-log-info "Unpacking package '${PACKAGE}'"
    local TMP_DIR=""
    TMP_DIR="$(mktemp -d)"
    tar -xf "${BB_RP_FETCHED_PACKAGES_STORE}/${DIGEST}" -C "${TMP_DIR}"

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
    rm -rf "${TMP_DIR}"

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
