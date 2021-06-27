#!/bin/bash

# Copyright 2021 Flant CJSC
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

. /etc/os-release

epel_package="epel-release"

if [[ "${ID}" == "rhel" ]]; then
  if ! rpm -q $epel_package >/dev/null; then
    epel_package="https://dl.fedoraproject.org/pub/epel/epel-release-latest-7.noarch.rpm"
  fi
fi

until yum install "$epel_package" -y; do
  echo "Error installing $epel_package"
  sleep 10
done
until yum install jq nc curl wget -y; do
  echo "Error installing packages"
  sleep 10
done

mkdir -p /var/lib/bashible/
