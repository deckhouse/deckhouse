#!/bin/bash
{{- /*
# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/}}
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
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  redos)
    case "$VERSION_ID" in 7|7.*)
      echo "redos" && exit 0 ;;
    esac
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  alteros)
    case "$VERSION_ID" in 7|7.*)
      echo "alteros" && exit 0 ;;
    esac
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  ubuntu)
    case "$VERSION_ID" in 18.04|20.04|22.04)
      echo "ubuntu-lts" && exit 0 ;;
    esac
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  debian)
    case "$VERSION_ID" in 9|10|11)
      echo "debian" && exit 0 ;;
    esac
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  astra)
    case "$VERSION_ID" in
      1.7|1.7*)
        echo "astra" && exit 0 ;;
      2.12|2.12.*)
        echo "debian" && exit 0 ;;
    esac
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  altlinux)
    case "$VERSION_ID" in p10)
      echo "altlinux" && exit 0 ;;
    esac
    >&2 echo "ERROR: ${PRETTY_NAME} is not supported."
    exit 1
  ;;
  "")
    >&2 echo "ERROR: Can't determine OS! No ID in /etc/os-release."
    exit 1
  ;;
esac
{{- /*
# try to determine os by ID_LIKE
*/}}
for ID in $ID_LIKE; do
  case "$ID" in
    centos|rhel)
      >&2 echo "WARNING: Trying to use centos bundle as default for: ${PRETTY_NAME}"
      echo "centos" && exit 0
    ;;
    debian)
      >&2 echo "WARNING: Trying to use debian bundle as default for: ${PRETTY_NAME}"
      echo "debian" && exit 0
    ;;
    altlinux)
      >&2 echo "WARNING: Trying to use altlinux bundle as default for: ${PRETTY_NAME}"
      echo "altlinux" && exit 0
    ;;
  esac
done
{{- /*
# try to determine os by packet manager
*/}}
bundle="debian"
if yum -q --version >/dev/null 2>/dev/null; then
  bundle="centos"
fi
>&2 echo "WARNING: Trying to use ${bundle} bundle as default for: ${PRETTY_NAME}"
echo "${bundle}"
