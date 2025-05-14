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
backup_dir_on_host=${HOSTPATH}
etcdctl \
    --endpoints=https://127.0.0.1:2379 \
    --cacert=/etc/kubernetes/pki/etcd/ca.crt \
    --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
    --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
    snapshot save "${etcd}"
tar -czvf "${archive}" "${etcd}"
# Check that there is enough free space
if [ $(df -B1 /var/backup | tail -1 | awk -v ETCDQUOTA="${ETCDQUOTA}" '{printf "%.0f\n", $4 - (ETCDQUOTA * 2)}') -ge 0 ]; then
    chmod 0600 "${archive}"
    mv "${archive}" "/var/backup/${archive}"
else
    echo "Free space in ${backup_dir_on_host} is too small for backup should be more than double size ETCDQUOTA (${ETCDQUOTA} bytes)"
    exit 1
fi
