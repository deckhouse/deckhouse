#!/bin/bash

set -Eeo pipefail

function error() {
  echo -n "not-ready" > /tmp/coredns-readiness
}

trap error ERR

curl -sS --connect-timeout 1 --max-time 1 "127.0.0.1:9225/health" >/dev/null
curl -sS --connect-timeout 1 --max-time 1 "127.0.0.1:9254/metrics" >/dev/null

# Check internal cluster DNS name
dig kubernetes.default.svc.${KUBE_CLUSTER_DOMAIN}. @169.254.20.10 +short +timeout=1 +tries=2 | grep -v -e '^$' >/dev/null

echo -n "ready" > /tmp/coredns-readiness
