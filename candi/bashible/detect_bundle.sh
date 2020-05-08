#!/bin/sh

if [ -e /etc/os-release ]; then
  . /etc/os-release
  bundleName="${ID}-${VERSION_ID}"
  case $bundleName in
    ubuntu-18.04|centos-7)
      echo $bundleName
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
