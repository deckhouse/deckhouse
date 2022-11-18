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
# shellcheck disable=SC2153

# check if package installed
# bb-rp-is-installed? package tag
bb-rp-is-installed?() {
  if [[ -d "${BB_RP_INSTALLED_PACKAGES_STORE}/${1}" ]]; then
    local INSTALLED_TAG=""
    INSTALLED_TAG="$(cat "${BB_RP_INSTALLED_PACKAGES_STORE}/${1}/tag")"
    if [[ "${INSTALLED_TAG}" == "${2}" ]]; then
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
# bb-rp-get-digests tag
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
  curl --retry 3 -sSLH "Authorization: Bearer ${TOKEN}" "${SCHEME}://${REGISTRY_ADDRESS}/v2${REGISTRY_PATH}/blobs/${1}" -o "${2}"
}

# download package digests, unpack them and run install script
# bb-rp-install package:tag
bb-rp-install() {
  for PACKAGE_WITH_TAG in "$@"; do
    local PACKAGE=""
    local TAG=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_TAG}")"
    TAG="$(awk -F ":" '{print $2}' <<< "${PACKAGE_WITH_TAG}")"

    # shellcheck disable=SC2211
    if bb-rp-is-installed? "${PACKAGE}" "${TAG}"; then
      continue
    fi

    local DIGESTS=""
    DIGESTS="$(bb-rp-get-digests "${TAG}")"

    local TMPDIR=""
    TMPDIR="$(mktemp -d)"

    # Get digests
    for DIGEST in ${DIGESTS}; do
      local TMPFILE=""
      TMPFILE="$(mktemp -u)"
      bb-rp-fetch-digest "${DIGEST}" "${TMPFILE}"
      tar -xf "${TMPFILE}" -C "${TMPDIR}"
      rm -f "${TMPFILE}"
    done

    bb-log-info "Installing package '${PACKAGE}'"
    # run install script
    # shellcheck disable=SC2164
    pushd "${TMPDIR}" >/dev/null
    ./install
     bb-exit-on-error "Failed to install package '${PACKAGE}'"
    # shellcheck disable=SC2164
    popd >/dev/null

    # Write tag to hold file
    mkdir -p "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    echo "${TAG}" > "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}/tag"
    # copy install/uninstall scripts to hold dir
    cp "${TMPDIR}/install" "${TMPDIR}/uninstall" "${BB_RP_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    # cleanup
    rm -rf "${TMPDIR}"

    bb-event-fire "bb-package-installed" "${PACKAGE}"
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
