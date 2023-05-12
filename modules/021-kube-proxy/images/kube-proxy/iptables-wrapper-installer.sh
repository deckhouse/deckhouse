#!/bin/sh

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
