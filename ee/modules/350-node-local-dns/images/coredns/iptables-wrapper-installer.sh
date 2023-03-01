#!/bin/bash

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

chroot_sbin="/dpkg/usr/sbin"
sbin="/usr/sbin"

# Start creating the wrapper...
rm -f "${chroot_sbin}/iptables-wrapper"
cat > "${chroot_sbin}/iptables-wrapper" <<EOF
#!/bin/bash

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

# NOTE: This can only use POSIX /bin/sh features; the container image
# might not contain bash.

set -eu

# In kubernetes 1.17 and later, kubelet will have created at least
# one chain in the "mangle" table (either "KUBE-IPTABLES-HINT" or
# "KUBE-KUBELET-CANARY"), so check that first, against
# iptables-nft, because we can check that more efficiently and
# it's more common these days.
nft_kubelet_rules=\$( (iptables-nft-save -t mangle || true; ip6tables-nft-save -t mangle || true) 2>/dev/null | grep -E '^:(KUBE-IPTABLES-HINT|KUBE-KUBELET-CANARY)' | wc -l)
if [ "\${nft_kubelet_rules}" -ne 0 ]; then
    mode=nft
else
    # Check for kubernetes 1.17-or-later with iptables-legacy. We
    # can't pass "-t mangle" to iptables-legacy-save because it would
    # cause the kernel to create that table if it didn't already
    # exist, which we don't want. So we have to grab all the rules
    legacy_kubelet_rules=\$( (iptables-legacy-save || true; ip6tables-legacy-save || true) 2>/dev/null | grep -E '^:(KUBE-IPTABLES-HINT|KUBE-KUBELET-CANARY)' | wc -l)
    if [ "\${legacy_kubelet_rules}" -ne 0 ]; then
        mode=legacy
    else
        # With older kubernetes releases there may not be any _specific_
        # rules we can look for, but we assume that some non-containerized process
        # (possibly kubelet) will have created _some_ iptables rules.
        num_legacy_lines=\$( (iptables-legacy-save || true; ip6tables-legacy-save || true) 2>/dev/null | grep '^-' | wc -l)
        num_nft_lines=\$( (iptables-nft-save || true; ip6tables-nft-save || true) 2>/dev/null | grep '^-' | wc -l)
        if [ "\${num_legacy_lines}" -gt "\${num_nft_lines}" ]; then
            mode=legacy
        else
            mode=nft
        fi
    fi
fi
EOF

# Write out the appropriate alternatives-selection commands
cat >> "${chroot_sbin}/iptables-wrapper" <<EOF
# Update links to point to the selected binaries
for cmd in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do
    rm -f "${sbin}/\${cmd}"
    ln -s "${sbin}/xtables-\${mode}-multi" "${sbin}/\${cmd}"
done 2>/dev/null || failed=1
EOF

# Write out the post-alternatives-selection error checking and final wrap-up
cat >> "${chroot_sbin}/iptables-wrapper" <<EOF
if [ "\${failed:-0}" = 1 ]; then
    echo "Unable to redirect iptables binaries. (Are you running in an unprivileged pod?)" 1>&2
    # fake it, though this will probably also fail if they aren't root
    exec "${sbin}/xtables-\${mode}-multi" "\$0" "\$@"
fi

# Now re-exec the original command with the newly-selected alternative
exec "\$0" "\$@"
EOF
chmod +x "${chroot_sbin}/iptables-wrapper"

# Now back in the installer script, point the iptables binaries at our
# wrapper
for cmd in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do
        rm -f "${chroot_sbin}/${cmd}"
        ln -s "${sbin}/iptables-wrapper" "${chroot_sbin}/${cmd}"
done

# Cleanup
rm -f "$0"
