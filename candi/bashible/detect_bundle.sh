#!/bin/bash
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
if [ -e /etc/os-release ]; then
  . /etc/os-release
  bundleName="${ID}-${VERSION_ID}"
  case $bundleName in
    \(centos|rocky|almalinux|rhel\)-\(7|7.*|8|8.*|9|9.*\))
      echo "centos"
      exit 0
    ;;
    ubuntu-\(16.04|18.04|20.04|22.04\))
      echo "ubuntu-lts"
      exit 0
    ;;
    debian-\(9|10|11\))
      echo "debian"
      exit 0
    ;;
    "-")
      >&2 echo "ERROR: Can't determine OS! No ID and VERSION_ID in /etc/os-release."
      exit 1
    ;;
  esac
  # try to determine os by packet manager
  bundle="debian"
  if yum -q --version >/dev/null 2>/dev/null; then
    bundle="centos"
  fi
  >&2 echo "WARNING: Trying to use ${bundle} bundle as default for: ${PRETTY_NAME}"
  echo "${bundle}"
  exit 0
fi

>&2 echo "ERROR: Can't determine OS! /etc/os-release is not found."
exit 1
