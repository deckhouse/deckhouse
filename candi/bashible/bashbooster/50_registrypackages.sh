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

# shellcheck disable=SC2211,SC2153

bb-var BB_INSTALLED_PACKAGES_STORE "/var/cache/registrypackages"
bb-var BB_FETCHED_PACKAGES_STORE "${TMPDIR}/registrypackages"

# check if package installed
# bb-package-is-installed? package digest
bb-package-is-installed?() {
  if [[ -d "${BB_INSTALLED_PACKAGES_STORE}/${1}" ]]; then
    local INSTALLED_DIGEST=""
    INSTALLED_DIGEST="$(cat "${BB_INSTALLED_PACKAGES_STORE}/${1}/digest")"
    if [[ "${INSTALLED_DIGEST}" == "${2}" ]]; then
      return 0
    fi
  fi
  return 1
}

# Check if package fetched
# bb-package-is-fetched? package digest
bb-package-is-fetched?() {
  if [[ -d "${BB_FETCHED_PACKAGES_STORE}/${1}" ]]; then
    if [[ -f "${BB_FETCHED_PACKAGES_STORE}/${1}/${2}.tar.gz" ]]; then
      return 0
    fi
  fi
  return 1
}

# Ckeck the python version
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

# Fetch a package using python.
# bb-package-proxy-fetch-blob digest output_file_path
bb-package-fetch-blob() {
  check_python

  cat - <<EOF | $python_binary
import random
import socket
import ssl

try:
    from urllib.request import urlretrieve, build_opener, install_opener
except ImportError as e:
    from urllib2 import urlretrieve, build_opener, install_opener

socket.setdefaulttimeout(60)

ssl._create_default_https_context = ssl._create_unverified_context

endpoints = "${PACKAGES_PROXY_ADDRESSES}".split(",")
token = "${PACKAGES_PROXY_TOKEN}"

# Choose a random endpoint to increase fault tolerance and reduce load on a single endpoint.
endpoint = random.choice(endpoints)

opener = build_opener()
opener.addheaders = [('Authorization', f'Bearer {token}')]
install_opener(opener)

url = f'https://{endpoint}/package?digest=$1&repository=${REPOSITORY}'
urlretrieve(url, "$2")
EOF
}

# Fetch digests from registry and save to file
# bb-package-fetch-blobs map[blob_digest]output_file_path [repository]
#
# This function uses the PACKAGES_MAP variable from the scope of the bb-package-fetch()
# due to the limitations of using `declare -n` in CentOS 7 (bash 4.2, and 4.3 is needed).
# DO NOT CALL THIS FUNCTION DIRECTLY!
bb-package-fetch-blobs() {
  local PACKAGE_DIGEST
  for PACKAGE_DIGEST in "${!PACKAGES_MAP[@]}"; do
    local PACKAGE_DIR="${BB_FETCHED_PACKAGES_STORE}/${PACKAGES_MAP[$PACKAGE_DIGEST]}"
    mkdir -p "${PACKAGE_DIR}"
    bb-package-fetch-blob "${PACKAGE_DIGEST}" "${PACKAGE_DIR}/${PACKAGE_DIGEST}.tar.gz"
  done
}

# Unpack packages and run install script
# bb-package-install package:digest
bb-package-install() {
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    if bb-package-is-installed? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package already installed"
      continue
    fi

    if ! bb-package-is-fetched? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package not found locally"
      bb-package-fetch "${PACKAGE_WITH_DIGEST}"
    fi

    bb-log-info "Unpacking package '${PACKAGE}'"
    local TMP_DIR=""
    TMP_DIR="$(mktemp -d)"
    trap '
      rm -rf "${TMP_DIR}" "${BB_FETCHED_PACKAGES_STORE:?}/${PACKAGE}"
      bb-log-error "Failed to unpack package "${PACKAGE}", it may be corrupted. The package will be refetched on the next attempt"
    ' ERR
    tar -xf "${BB_FETCHED_PACKAGES_STORE}/${PACKAGE}/${DIGEST}.tar.gz" -C "${TMP_DIR}"
    trap - ERR

    bb-log-info "Installing package '${PACKAGE}'"
    # shellcheck disable=SC2164
    pushd "${TMP_DIR}" >/dev/null
    trap '
      popd >/dev/null
      rm -rf "${TMP_DIR}"
      bb-log-error "Failed to install package "${PACKAGE}""
    ' ERR
    ./install
    trap - ERR
    popd >/dev/null

    # Write digest to hold file
    mkdir -p "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    echo "${DIGEST}" > "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}/digest"
    # Copy install/uninstall scripts to hold dir
    cp "${TMP_DIR}/install" "${TMP_DIR}/uninstall" "${BB_INSTALLED_PACKAGES_STORE}/${PACKAGE}"
    # Cleanup
    rm -rf "${TMP_DIR}" "${BB_FETCHED_PACKAGES_STORE:?}/${PACKAGE}"

    bb-log-info "'${PACKAGE}' package successfully installed"
    bb-event-fire "bb-package-installed" "${PACKAGE}"
    trap - ERR
  done
}

# Unpack package from module image and run install script
# bb-package-module-install package:digest repository
bb-package-module-install() {
  local MODULE_PACKAGE=$1
  local REPOSITORY=$2

  bb-package-install $MODULE_PACKAGE
}

# Fetch packages by digest
# bb-package-fetch package1:digest1 [package2:digest2 ...]
bb-package-fetch() {
  mkdir -p "${BB_FETCHED_PACKAGES_STORE}"

  declare -A PACKAGES_MAP
  local PACKAGE_WITH_DIGEST
  for PACKAGE_WITH_DIGEST in "$@"; do
    local PACKAGE=""
    local DIGEST=""
    PACKAGE="$(awk -F ":" '{print $1}' <<< "${PACKAGE_WITH_DIGEST}")"
    DIGEST="$(awk -F ":" '{print $2":"$3}' <<< "${PACKAGE_WITH_DIGEST}")"

    if bb-package-is-installed? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package already installed"
      continue
    fi

    if bb-package-is-fetched? "${PACKAGE}" "${DIGEST}"; then
      bb-log-info "'${PACKAGE_WITH_DIGEST}' package already fetched"
      continue
    fi

    PACKAGES_MAP[$DIGEST]="${PACKAGE}"
  done

  if [ "${#PACKAGES_MAP[@]}" -eq 0 ]; then
    return 0
  fi


  bb-log-info "Fetching packages: ${PACKAGES_MAP[*]}"
  trap 'bb-log-error "Failed to fetch packages"' ERR

  bb-package-fetch-blobs PACKAGES_MAP
  trap - ERR
  bb-log-info "Packages saved under ${BB_FETCHED_PACKAGES_STORE}"
}

# Unpack package from module image and run install script
# bb-package-module-install package:digest registry_auth scheme registry_address registry_path
bb-package-module-install() {
  local MODULE_PACKAGE=$1
  local REGISTRY_AUTH=$2
  local SCHEME=$3
  local REGISTRY_ADDRESS=$4
  local REGISTRY_PATH=$5

  bb-package-install $MODULE_PACKAGE
}

# run uninstall script from hold dir
# bb-package-remove package
bb-package-remove() {
  for PACKAGE in "$@"; do
    if [[ -f "${BB_INSTALLED_PACKAGES_STORE:?}/${PACKAGE:?}/uninstall" ]]; then
      bb-log-info "Removing package '${PACKAGE}'"
      # shellcheck disable=SC1090
      . "${BB_INSTALLED_PACKAGES_STORE:?}/${PACKAGE:?}/uninstall"
      bb-exit-on-error "Failed to remove package '${PACKAGE}'"
      # cleanup
      rm -rf "${BB_INSTALLED_PACKAGES_STORE:?}/${PACKAGE:?}"
      bb-event-fire "bb-package-removed" "${PACKAGE}"
    fi
  done
}
