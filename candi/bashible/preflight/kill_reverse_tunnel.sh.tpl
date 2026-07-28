#!/usr/bin/env bash
{{- /*
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
*/}}

# Kill the process listening on a TCP port by resolving its socket through /proc.

# Build the little-endian hexadecimal address used in /proc/net/tcp,
# for example: 1.2.3.4:80 -> 04030201:0050.
hex_addr="$(awk -v ip='{{.host}}' -v port='{{.port}}' 'BEGIN {
    split(ip, b, ".")
    printf "%02X%02X%02X%02X:%04X", b[4], b[3], b[2], b[1], port
}')"

# Find the socket inode of the matching LISTEN entry. TCP state 0A means LISTEN.
inode="$(awk -v address="$hex_addr" '
    $2 == address && $4 == "0A" {
        print $10
        exit
    }
' /proc/net/tcp)"

if [ -z "$inode" ]; then
    echo "Port not found"
    exit 0
fi

# Map the socket inode to a PID through /proc/<pid>/fd and kill the process.
for fd in /proc/[0-9]*/fd/*; do
    [ "$(readlink "$fd" 2>/dev/null)" = "socket:[$inode]" ] || continue

    pid="${fd#/proc/}"
    pid="${pid%%/*}"

    echo "$pid"
    kill -9 "$pid" 2>/dev/null
    exit 0
done

echo "Port not found"
exit 0
