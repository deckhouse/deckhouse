#!/bin/sh
#
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

mode=$1

case "${mode}" in
    legacy)
        wrongmode=nft
        ;;
    nft)
        wrongmode=legacy
        ;;
    *)
        echo "ERROR: bad mode '${mode}'" 1>&2
        exit 1
        ;;
esac

if [ -d /usr/sbin -a -e /usr/sbin/iptables ]; then
    sbin="/usr/sbin"
elif [ -d /sbin -a -e /sbin/iptables ]; then
    sbin="/sbin"
else
    echo "ERROR: iptables is not present in either /usr/sbin or /sbin" 1>&2
    exit 1
fi

ensure_iptables_undecided() {
    iptables=$(realpath "${sbin}/iptables")
    if [ "${iptables}" != "${sbin}/iptables-wrapper" ]; then
	echo "iptables link was resolved prematurely! (${iptables})" 1>&2
	exit 1
    fi
}

ensure_iptables_resolved() {
    expected=$1
    iptables=$(realpath "${sbin}/iptables")
    if [ "${iptables}" = "${sbin}/iptables-wrapper" ]; then
	echo "iptables link is not yet resolved!" 1>&2
	exit 1
    fi
    version=$(iptables -V | sed -e 's/.*(\(.*\)).*/\1/')
    case "${version}/${expected}" in
	legacy/legacy|nf_tables/nft)
	    return
	    ;;
	*)
	    echo "iptables link resolved incorrectly (expected ${expected}, got ${version})" 1>&2
	    exit 1
	    ;;
    esac
}

ensure_iptables_undecided

# Initialize the chosen iptables mode with just a hint chain
iptables-${mode} -t mangle -N KUBE-IPTABLES-HINT

# Put some junk in the other iptables system
iptables-${wrongmode} -t filter -N BAD-1
iptables-${wrongmode} -t filter -A BAD-1 -j ACCEPT
iptables-${wrongmode} -t filter -N BAD-2
iptables-${wrongmode} -t filter -A BAD-2 -j DROP

ensure_iptables_undecided

iptables -L > /dev/null

ensure_iptables_resolved ${mode}
