#!/bin/sh

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

set -xe
cd /tmp/
etcd=etcd-backup.snapshot
archive=etcd-backup.tar.gz
etcdctl \
    --endpoints=https://127.0.0.1:2379 \
    --cacert=/etc/kubernetes/pki/etcd/ca.crt \
    --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
    --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
    snapshot save "${etcd}"
tar -czvf "${archive}" "${etcd}"
# Check that there will be 25% free space left after adding the file
if [ $(df /var/lib/etcd/ | tail -1 | awk '{printf "%.0f\n", $4 - ($2 * 0.25)}') -ge $(du -k "${archive}" | awk '{print $1}') ]; then
    chmod 0600 "${archive}"
    mv "${archive}" "/var/lib/etcd/${archive}"
else
    echo "Free space in /var/lib/etcd/ is too small for backup should be more than 25%"
    exit 1
fi
