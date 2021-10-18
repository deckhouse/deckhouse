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
bb-var BB_RP_PREFIX "${REGISTRY_PATH%/*}/binaries"

# check if image installed
# bb-rp-is-installed? image tag
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
# bb-rp-get-token image
 bb-rp-get-token() {
  local AUTH=""
  local AUTH_HEADER=""
  local AUTH_REALM=""
  local AUTH_SERVICE=""

  if [[ -n ${REGISTRY_AUTH} ]]; then
    AUTH="-u ${REGISTRY_AUTH}"
  fi

  AUTH_HEADER="$(curl --retry 3 -sSLi "${SCHEME}://${REGISTRY_ADDRESS}/v2/" | grep -i "www-authenticate")"
  AUTH_REALM="$(awk -F "," '{split($1,s,"\""); print s[2]}' <<< "${AUTH_HEADER}")"
  AUTH_SERVICE="$(awk -F "," '{split($2,s,"\""); print s[2]}' <<< "${AUTH_HEADER}" | sed "s/ /+/g")"
  # shellcheck disable=SC2086
  # Remove leading / from BB_RP_PREFIX due to scope format -> scope=repository:sys/binaries/jq:pull
  curl --retry 3 -fsSL ${AUTH} "${AUTH_REALM}?service=${AUTH_SERVICE}&scope=repository:${BB_RP_PREFIX#/}/${1}:pull" | jq -r '.token'
}

# fetch manifest from registry and get list of digests
# bb-rp-get-digests crictl v1.19
bb-rp-get-digests() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token "${1}")"
  curl --retry 3 -fsSL \
			-H "Authorization: Bearer ${TOKEN}" \
			-H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
			"${SCHEME}://${REGISTRY_ADDRESS}/v2${BB_RP_PREFIX}/${1}/manifests/${2}" | jq -r '.layers[].digest'
}

# Fetch digest from registry
# bb-rp-fetch-digest image digest outfile
bb-rp-fetch-digest() {
  local TOKEN=""
  TOKEN="$(bb-rp-get-token "${1}")"
  curl --retry 3 -sSLH "Authorization: Bearer ${TOKEN}" "${SCHEME}://${REGISTRY_ADDRESS}/v2${BB_RP_PREFIX}/${1}/blobs/${2}" -o "${3}"
}

# download image digests, unpack them and run install script
# bb-rp-install crictl:v1.19
bb-rp-install() {
  for IMAGE_WITH_TAG in "$@"; do
    local IMAGE=""
    local TAG=""
    IMAGE="$(awk -F ":" '{print $1}' <<< "${IMAGE_WITH_TAG}")"
    TAG="$(awk -F ":" '{print $2}' <<< "${IMAGE_WITH_TAG}")"

    # shellcheck disable=SC2211
    if bb-rp-is-installed? "${IMAGE}" "${TAG}"; then
      continue
    fi

    local DIGESTS=""
    DIGESTS="$(bb-rp-get-digests "${IMAGE}" "${TAG}")"

    local TMPDIR=""
    TMPDIR="$(mktemp -d)"

    # Get digests
    for DIGEST in ${DIGESTS}; do
      local TMPFILE=""
      TMPFILE="$(mktemp -u)"
      bb-rp-fetch-digest "${IMAGE}" "${DIGEST}" "${TMPFILE}"
      tar -xf "${TMPFILE}" -C "${TMPDIR}"
      rm -f "${TMPFILE}"
    done

    bb-log-info "Installing package '${IMAGE_WITH_TAG}'"
    # run install script
    # shellcheck disable=SC2164
    pushd "${TMPDIR}" >/dev/null
    ./install
     bb-exit-on-error "Failed to install package '${IMAGE_WITH_TAG}'"
    # shellcheck disable=SC2164
    popd >/dev/null

    # Write tag to hold file
    mkdir -p "${BB_RP_INSTALLED_PACKAGES_STORE}/${IMAGE}"
    echo "${TAG}" > "${BB_RP_INSTALLED_PACKAGES_STORE}/${IMAGE}/tag"
    # copy install/uninstall scripts to hold dir
    cp "${TMPDIR}/install" "${TMPDIR}/uninstall" "${BB_RP_INSTALLED_PACKAGES_STORE}/${IMAGE}"
    # cleanup
    rm -rf "${TMPDIR}"

    bb-event-fire "bb-package-installed" "${IMAGE_WITH_TAG}"
  done
}

# run uninstall script from hold dir
# bb-rp-remove crictl
bb-rp-remove() {
  for IMAGE in "$@"; do
    if [[ -f "${BB_RP_INSTALLED_PACKAGES_STORE:?}/${IMAGE:?}/uninstall" ]]; then
      bb-log-info "Removing package '${IMAGE}'"
      # shellcheck disable=SC1090
      . "${BB_RP_INSTALLED_PACKAGES_STORE:?}/${IMAGE:?}/uninstall"
      bb-exit-on-error "Failed to remove package '${IMAGE}'"
      # cleanup
      rm -rf "${BB_RP_INSTALLED_PACKAGES_STORE:?}/${IMAGE:?}"
      bb-event-fire "bb-package-removed" "${IMAGE}"
    fi
  done
}
