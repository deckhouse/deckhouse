#!/bin/bash

# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

set -Eeuo pipefail

function error() {
  echo -n "not-ready" > /tmp/coredns-readiness
  exit 1
}

trap error ERR

curl -sS --connect-timeout 1 --max-time 1 "127.0.0.1:9225/health" >/dev/null
curl -sS --connect-timeout 1 --max-time 1 "127.0.0.1:4224/metrics" >/dev/null

# Check internal cluster DNS name
dig "kubernetes.default.svc.${KUBE_CLUSTER_DOMAIN}." @169.254.20.10 +short +timeout=1 +tries=2 | grep -v -e '^$' >/dev/null

echo -n "ready" > /tmp/coredns-readiness
