#!/bin/bash

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

bb-is-bundle(){
  local os=""

  if [ ! -e /etc/os-release ]; then
    bb-exit 1 "ERROR: Can't determine OS! /etc/os-release is not found."
  fi

  . /etc/os-release
  case "$ID" in
    centos|rocky|almalinux|rhel|ol)
      case "$VERSION_ID" in 7*|8*|9*)
        os="centos" ;;
      esac
    ;;
    redos)
      case "$VERSION_ID" in 7*|8*)
        os="redos" ;;
      esac
    ;;
    rels|rosa)
      case "$VERSION_ID" in 7.9)
        os="rosa" ;;
      esac
      case "$VERSION" in 12.4|12.5.*|12.6|12.6.*)
        os="rosa" ;;
      esac
    ;;
    ubuntu)
      case "$VERSION_ID" in 18.04|20.04|22.04|24.04)
        os="ubuntu-lts" ;;
      esac
    ;;
    debian)
      case "$VERSION_ID" in 10|11|12|13)
        os="debian" ;;
      esac
    ;;
    astra)
      case "$VERSION_ID" in
        1.7*|1.8*)
          os="astra" ;;
        2.12*)
          os="debian" ;;
      esac
    ;;
    altlinux)
      case "$VERSION_ID" in p10|10|10.0|10.1|10.2|11)
          os="altlinux" ;;
      esac
    ;;
    mosos-arbat|opensuse-leap)
      case "$VERSION" in 15.*)
          os="opensuse" ;;
      esac
    ;;
    "")
      bb-exit 1 "ERROR: Can't determine OS! No ID in /etc/os-release."
    ;;
  esac

  if [ -n "$os" ]; then
    echo "$os"
  else
    bb-exit 1 "ERROR: ${PRETTY_NAME} is not supported."
  fi
}
