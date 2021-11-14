#!/bin/bash
{{- /*
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
*/}}
if [ -e /etc/os-release ]; then
  . /etc/os-release
  bundleName="${ID}-${VERSION_ID}"
  case $bundleName in
    centos-7|rhel-7.*)
      echo "centos-7"
      exit 0
    ;;
    ubuntu-16.04|ubuntu-18.04|ubuntu-20.04)
      echo "ubuntu-lts"
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
