#!/bin/sh
set -xe
cd /tmp/
etcd=etcd-backup.snapshot
archive=etcd-backup.tar.gz
etcdctl --endpoints=https://127.0.0.1:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt --key=/etc/kubernetes/pki/etcd/healthcheck-client.key snapshot save "${etcd}"
tar -czvf "${archive}" "${etcd}"
# Check that there will be 30% free space left after adding the file.
if [ $(df /var/lib/etcd/ | tail -1 | awk '{printf "%.0f\n", $4 - ($2 * 0.3)}') -ge $(du -k "${archive}" | awk '{print $1}') ]; then
    cp "${archive}" "/var/lib/etcd/${archive}"
else
    echo "Free space in /var/lib/etcd/ is too small for backup should be more than 30%."
    exit 1
fi
