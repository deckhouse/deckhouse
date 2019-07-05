#!/usr/bin/env bash

set -Eeuo pipefail

# `dig` returns non-zero exit code only when there is a server failure (SERVFAIL),
# it won't return non-zero exit code on NXDOMAIN.
# Here we generate a random, certain-to-not-be-in-cache DNS request.
dig "$(head /dev/urandom | tr -dc A-Za-z0-9 | head -c10).default.svc.cluster.local." @"$KUBE_DNS_SVC_IP" +short +timeout=2 +tries=2

curl -sS "127.0.0.1:9225/health"
