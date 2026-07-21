#!/usr/bin/env bash
# Mirror logic from modules/040-node-manager/templates/node-group/_bootstrap.tpl
set -Eeuo pipefail
export BOOTSTRAP_DIR=/var/lib/bashible
export TMPDIR=/opt/deckhouse/tmp
mkdir -p "$BOOTSTRAP_DIR" "$TMPDIR"
chmod 0700 "$BOOTSTRAP_DIR"

# VCP-specific: resolve the SNI hostnames to the ALB, then seed token + tenant CA.
grep -q "${VCP_API_HOST}" /etc/hosts || \
  echo "${VCP_ALB_VIP} ${VCP_API_HOST} ${VCP_KONN_HOST} ${VCP_PKG_HOST}" >> /etc/hosts
printf '%s' "${VCP_JOIN_TOKEN}" > "$BOOTSTRAP_DIR/bootstrap-token"
chmod 0600 "$BOOTSTRAP_DIR/bootstrap-token"
echo -n "${VCP_CA_CRT_B64}" | base64 -d > "$BOOTSTRAP_DIR/ca.crt"

touch "$BOOTSTRAP_DIR/first_run"
