#!/bin/bash

# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

function name_is_not_supported() {
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
}

function try_bundle(){
  >&2 echo "WARNING: Trying to use ${1} bundle as default for: ${PRETTY_NAME}"
  echo "${1}"
  exit 0
}

if [ ! -e /etc/os-release ]; then
  >&2 echo "ERROR: Can't determine OS! /etc/os-release is not found."
  exit 1
fi

. /etc/os-release
case "$ID" in
  centos|rocky|almalinux|rhel)
    case "$VERSION_ID" in 7|7.*|8|8.*|9|9.*)
      echo "centos" && exit 0 ;;
    esac
    name_is_not_supported
  ;;
  redos)
    case "$VERSION_ID" in 7|7.*)
      echo "redos" && exit 0 ;;
    esac
    name_is_not_supported
  ;;
  ubuntu)
    case "$VERSION_ID" in 18.04|20.04|22.04|24.04)
      echo "ubuntu-lts" && exit 0 ;;
    esac
    name_is_not_supported
  ;;
  debian)
    case "$VERSION_ID" in 10|11|12)
      echo "debian" && exit 0 ;;
    esac
    name_is_not_supported
  ;;
  astra)
    case "$VERSION_ID" in
      1.7|1.7*|1.8|1.8*)
        echo "astra" && exit 0 ;;
      2.12|2.12.*)
        echo "debian" && exit 0 ;;
    esac
    name_is_not_supported
  ;;
  altlinux)
    case "$VERSION_ID" in p10|10|10.0|10.1|10.2)
      echo "altlinux" && exit 0 ;;
    esac
    name_is_not_supported
  ;;
  "")
    >&2 echo "ERROR: Can't determine OS! No ID in /etc/os-release."
    exit 1
  ;;
esac

# try to determine os by ID_LIKE
for ID in $ID_LIKE; do
  case "$ID" in
    centos|rhel)
      try_bundle "centos"
    ;;
    debian)
      try_bundle "debian"
    ;;
    altlinux)
      try_bundle "altlinux"
    ;;
  esac
done

# try to determine os by packet manager
bundle="debian"
if yum -q --version >/dev/null 2>/dev/null; then
  bundle="centos"
fi
try_bundle "${bundle}"
