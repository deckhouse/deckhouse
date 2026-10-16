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

# What was found, and nothing else. The way out is dhctl's to print: it owns the report the
# operator reads, so advice from here arrived as a paragraph glued onto the end of one line of it.
fail() {
    echo "$1"
    exit 1
}

uid="$(id -u deckhouse 2>/dev/null || true)"
primary_gid="$(id -g deckhouse 2>/dev/null || true)"
group_gid="$(getent group deckhouse | cut -d: -f3 || true)"
uid_owner="$(getent passwd "$EXPECTED_ID" | cut -d: -f1 || true)"
gid_owner="$(getent group "$EXPECTED_ID" | cut -d: -f1 || true)"

if [ -n "$uid_owner" ] && [ "$uid_owner" != "deckhouse" ]; then
    fail "UID ${EXPECTED_ID} is already assigned to user ${uid_owner}"
fi

if [ -n "$gid_owner" ] && [ "$gid_owner" != "deckhouse" ]; then
    fail "GID ${EXPECTED_ID} is already assigned to group ${gid_owner}"
fi

if [ -z "$uid" ] && [ -z "$group_gid" ]; then
    exit 0
fi

if [ "$uid" != "$EXPECTED_ID" ] || [ "$primary_gid" != "$EXPECTED_ID" ] || [ "$group_gid" != "$EXPECTED_ID" ]; then
    fail "deckhouse user or group exists with unexpected id: uid=${uid}, primary_gid=${primary_gid}, group_gid=${group_gid} (expected ${EXPECTED_ID})"
fi

if sudo -l -U deckhouse 2>/dev/null | grep -q "(ALL"; then
    fail "deckhouse user has sudo privileges, this is a security risk"
fi
