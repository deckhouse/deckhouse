#!/usr/bin/env bash
set -euo pipefail

PKI_DIR="${PWD}/pki"
SECRET_NAME="stable-identity-etcd-witness-secret"
NAMESPACE="kube-system"

kubectl -n "${NAMESPACE}" create secret generic "${SECRET_NAME}" \
  --from-file=ca.crt="${PKI_DIR}/ca.crt" \
  --from-file=peer.crt="${PKI_DIR}/peer.crt" \
  --from-file=peer.key="${PKI_DIR}/peer.key" \
  --from-file=server.crt="${PKI_DIR}/server.crt" \
  --from-file=server.key="${PKI_DIR}/server.key" \
  --dry-run=client -o yaml > secret.yaml
