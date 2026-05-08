#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export ENDPOINTS="${ENDPOINTS:-https://10.241.32.6:2379,https://10.241.36.30:2379}"
export CERT="${CERT:-/etc/kubernetes/pki/apiserver-etcd-client.crt}"
export KEY="${KEY:-/etc/kubernetes/pki/apiserver-etcd-client.key}"
export CACERT="${CACERT:-/etc/kubernetes/pki/etcd/ca.crt}"
export BENCH="${BENCH:-benchmark}"

etcdctl() {
  d8 k -n kube-system exec pod/etcd-mmazin-master-0 -- \
    etcdctl \
    --cacert /etc/kubernetes/pki/etcd/ca.crt \
    --cert /etc/kubernetes/pki/etcd/ca.crt \
    --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379 \
    "$@"
}

bench() {
  "${BENCH}" \
    --auto-sync-interval=0 \
    --endpoints="${ENDPOINTS}" \
    --cert="${CERT}" \
    --key="${KEY}" \
    --cacert="${CACERT}" \
    "$@"
}

healthcheck() {
  etcdctl endpoint health
  etcdctl endpoint status -w table
}

