#!/bin/bash
# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
if [ -e /etc/os-release ]; then
  . /etc/os-release
  bundleName="${ID}-${VERSION_ID}"
  case $bundleName in
    centos-7|rhel-7.*|centos-8|rhel-8.*)
      echo "centos"
      exit 0
    ;;
    ubuntu-16.04|ubuntu-18.04|ubuntu-20.04)
      echo "ubuntu-lts"
      exit 0
    ;;
    debian-9|debian-10|debian-11)
      echo "debian"
      exit 0
    ;;
    astra-2.12.*|astra-1.7*)
      echo "astra"
      exit 0
    ;;
    "-")
      >&2 echo "ERROR: Can't determine OS! No ID and VERSION_ID in /etc/os-release."
      exit 1
    ;;
    *)
      >&2 echo "ERROR: Unsupported Linux version: ${PRETTY_NAME}"
      exit 1
    ;;
  esac
fi

>&2 echo "ERROR: Can't determine OS! /etc/os-release is not found."
exit 1
