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

bb-is-ubuntu-version?() {
  local UBUNTU_VERSION=$1
  source /etc/os-release
  if [ "${VERSION_ID}" == "${UBUNTU_VERSION}" ] ; then
    return 0
  else
    return 1
  fi
}

bb-is-centos-version?() {
  local CENTOS_VERSION=$1
  source /etc/os-release
  if [[ "${VERSION_ID}" =~ ^${CENTOS_VERSION}.*$ ]] ; then
    return 0
  else
    return 1
  fi
}

bb-is-debian-version?() {
  local DEBIAN_VERSION=$1
  source /etc/os-release
  if [ "${VERSION_ID}" == "${DEBIAN_VERSION}" ] ; then
    return 0
  else
    return 1
  fi
}

bb-is-altlinux-version?() {
  local ALTLINUX_VERSION=$1
  source /etc/os-release
  if [[ "${VERSION_ID}" =~ ^${ALTLINUX_VERSION}.*$ ]] ; then
    return 0
  else
    return 1
  fi
}

bb-is-distro-like?() {
  local DISTRO_LIKE=$1
  source /etc/os-release
  # match only whole words
  if grep -q " ${DISTRO_LIKE} " <<< " $ID $ID_LIKE "; then
    return 0
  else
    return 1
  fi
}
