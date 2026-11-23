#!/bin/bash

# Copyright 2024 Flant JSC
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

# Usage: HOSTPATH=/var/lib/etcd NODEIP=<master-ip> /entrypoint.sh

set -euo pipefail

# Leave 1 GiB of headroom for etcd growth and archive overhead.
readonly RESERVE_BYTES=1073741824
readonly SNAPSHOT=etcd-backup.snapshot
readonly BACKUP_DIR=/var/backup
readonly ARCHIVE="$BACKUP_DIR/etcd-backup.tar.gz"
readonly TEMP_ARCHIVE="$ARCHIVE.tmp"

fail() {
    echo "etcd backup (${HOSTPATH:-$BACKUP_DIR}): $*" >&2
    exit 1
}

etcd_command() {
    etcdctl \
        --endpoints="https://${NODEIP}:2379" \
        --cacert=/etc/kubernetes/pki/etcd/ca.crt \
        --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
        --key=/etc/kubernetes/pki/etcd/healthcheck-client.key "$@"
}

require_space() {
    local path=$1 required_bytes=$2
    local disk_space available_bytes

    disk_space=$(df -B1 --output=avail "$path") ||
        fail "cannot determine free space in $path"
    available_bytes=$(printf '%s\n' "$disk_space" | awk 'NR == 2 {print $1}')
    printf 'etcd backup: path=%s available=%s required=%s\n' \
        "$path" "$available_bytes" "$required_bytes"
    [ "$available_bytes" -ge "$required_bytes" ] ||
        fail "insufficient or invalid free space in $path:" \
            "available=$available_bytes required=$required_bytes"
}

umask 077
[ -n "${NODEIP:-}" ] || fail "NODEIP is required"
sleep "${BACKUP_DELAY_SECONDS:-0}" || fail "backup delay failed"
cd /tmp || fail "cannot enter /tmp"

# Reclaim space left by an interrupted backup before checking free space.
rm -f -- "$TEMP_ARCHIVE" || fail "temporary archive cleanup failed"

status=$(
    etcd_command --command-timeout=10s endpoint status --write-out=fields
) || fail "cannot obtain etcd endpoint status from ${NODEIP}"

db_size=$(
    printf '%s\n' "$status" |
        awk -F '[[:space:]]*:[[:space:]]*' '$1 == "\"DBSize\"" {print $2}'
)
[ "$db_size" -gt 0 ] || fail "invalid DBSize from ${NODEIP}: '$db_size'"

echo "etcd backup: endpoint=$NODEIP dbSize=$db_size"

require_space /tmp "$((db_size + RESERVE_BYTES))"
# Budget for both files in case /tmp and the backup share free space.
require_space "$BACKUP_DIR" "$((2 * db_size + RESERVE_BYTES))"

etcd_command snapshot save "$SNAPSHOT" || fail "snapshot creation failed"

trap 'rm -f -- "$TEMP_ARCHIVE"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

tar -cf - "$SNAPSHOT" | gzip > "$TEMP_ARCHIVE" ||
    fail "archive creation failed"
# Rename within the backup directory to publish the archive atomically.
mv -T -- "$TEMP_ARCHIVE" "$ARCHIVE" || fail "archive publication failed"
