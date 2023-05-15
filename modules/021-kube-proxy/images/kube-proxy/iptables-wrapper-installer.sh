#!/bin/sh

# Copyright 2020 The Kubernetes Authors.
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

set -eu

sbin="$1"
iptables_wrapper_path="$2"

if [ ! -f "${iptables_wrapper_path}" ]; then
    echo "ERROR: iptables-wrapper is not present, expected at ${iptables_wrapper_path}" 1>&2
    exit 1
fi

for cmd in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do
        rm -f "${sbin}/${cmd}"
        ln -s "${iptables_wrapper_path}" "${sbin}/${cmd}"
done
