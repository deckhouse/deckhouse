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

# Usage:
#
#   iptables-wrapper-installer.sh [--no-sanity-check]
#
# Installs a wrapper iptables script in a container that will figure out
# whether iptables-legacy or iptables-nft is in use on the host and then
# replaces itself with the correct underlying iptables version.
#
# Unless "--no-sanity-check" is passed, it will first verify that the
# container already contains a suitable version of iptables.

# NOTE: This can only use POSIX /bin/sh features; the build container
# might not contain bash.

set -eu

iptables_wrapper_path="./iptables-wrapper"

# Verify that the iptables-wrapper bin has been copied alongside the installer script
if [ ! -f "${iptables_wrapper_path}" ]; then
    echo "ERROR: iptables-wrapper is not present, expected at ${iptables_wrapper_path}" 1>&2
    exit 1
fi


# Find iptables binary location
if [ -d /usr/sbin -a -e /usr/sbin/iptables ]; then
    sbin="/usr/sbin"
elif [ -d /sbin -a -e /sbin/iptables ]; then
    sbin="/sbin"
else
    echo "ERROR: iptables is not present in either /usr/sbin or /sbin" 1>&2
    exit 1
fi

# Determine how the system selects between iptables-legacy and iptables-nft
if [ -x /usr/sbin/alternatives ]; then
    # Fedora/SUSE style alternatives
    altstyle="fedora"
elif [ -x /usr/sbin/update-alternatives ]; then
    # Debian style alternatives
    altstyle="debian"
else
    # No alternatives system
    altstyle="none"
fi

if [ "${1:-}" != "--no-sanity-check" ]; then
    # Ensure dependencies are installed
    if ! version=$("${sbin}/iptables-nft" --version 2> /dev/null); then
        echo "ERROR: iptables-nft is not installed" 1>&2
        exit 1
    fi
    if ! "${sbin}/iptables-legacy" --version > /dev/null 2>&1; then
        echo "ERROR: iptables-legacy is not installed" 1>&2
        exit 1
    fi

    case "${version}" in
    *v1.8.[0123]\ *)
        echo "ERROR: iptables 1.8.0 - 1.8.3 have compatibility bugs." 1>&2
        echo "       Upgrade to 1.8.4 or newer." 1>&2
        exit 1
        ;;
    *)
        # 1.8.4+ are OK
        ;;
    esac
fi

# Copy the wrapper.
rm -f "${sbin}/iptables-wrapper"
mv "${iptables_wrapper_path}" "${sbin}/iptables-wrapper"

# Now back in the installer script, point the iptables binaries at our
# wrapper
case "${altstyle}" in
    fedora)
	alternatives \
            --install /usr/sbin/iptables iptables /usr/sbin/iptables-wrapper 100 \
            --slave /usr/sbin/iptables-restore iptables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/iptables-save iptables-save /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables iptables /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables-restore iptables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables-save iptables-save /usr/sbin/iptables-wrapper
	;;

    debian)
	update-alternatives \
            --install /usr/sbin/iptables iptables /usr/sbin/iptables-wrapper 100 \
            --slave /usr/sbin/iptables-restore iptables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/iptables-save iptables-save /usr/sbin/iptables-wrapper
	update-alternatives \
            --install /usr/sbin/ip6tables ip6tables /usr/sbin/iptables-wrapper 100 \
            --slave /usr/sbin/ip6tables-restore ip6tables-restore /usr/sbin/iptables-wrapper \
            --slave /usr/sbin/ip6tables-save ip6tables-save /usr/sbin/iptables-wrapper
	;;

    *)
	for cmd in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do
            rm -f "${sbin}/${cmd}"
            ln -s "${sbin}/iptables-wrapper" "${sbin}/${cmd}"
	done
	;;
esac

# Cleanup
rm -f "$0"
