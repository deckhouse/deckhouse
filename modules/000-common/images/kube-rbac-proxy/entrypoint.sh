#!/bin/sh

if [[ "${KUBE_RBAC_PROXY_LISTEN_ADDRESS}x" == "x" ]]; then
  >&2 echo "ERROR: environment variable KUBE_RBAC_PROXY_LISTEN_ADDRESS is empty"
  sleep 2
  exit 1
fi

exec ./kube-rbac-proxy $@
