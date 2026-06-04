#!/usr/bin/env bash
{{- /*
# Copyright 2025 Flant JSC
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
    
EXPECTED_ID="64535"

fail() {
    echo "ERROR: $1"
    echo ""
    echo "To resolve this issue:"
    echo "  - If the node was previously part of a Deckhouse cluster, run the cleanup script:"
    echo "      chmod +x /var/lib/bashible/cleanup_static_node.sh"
    echo "      sudo bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing"
    echo "  - Otherwise, remove the conflicting account manually:"
    echo "      sudo userdel deckhouse"
    echo "      sudo groupdel deckhouse"
    exit 1
}

uid="$(id -u deckhouse 2>/dev/null || true)"
gid="$(getent group deckhouse | cut -d: -f3 || true)"

if [ -z "$uid" ] && [ -z "$gid" ]; then
    exit 0
fi

if [ "$uid" != "$EXPECTED_ID" ] || [ "$gid" != "$EXPECTED_ID" ]; then
    fail "deckhouse user or group exists with unexpected id: uid=${uid}, gid=${gid} (expected ${EXPECTED_ID})"
fi

if sudo -l -U deckhouse 2>/dev/null | grep -q "(ALL"; then
    fail "deckhouse user has sudo privileges — this is a security risk"
fi