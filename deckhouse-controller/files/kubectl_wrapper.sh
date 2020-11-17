#!/bin/bash

set -Eeuo pipefail

if [ -s /tmp/kubectl_version ]; then
 kubernetes_version="$(cat /tmp/kubectl_version)"
else
 # Workaround for running kubectl before global hook global-hooks/discovery/kubernetes_version running
 kubernetes_version="$(/usr/local/bin/kubectl-1.16 version -o json | jq -r '.serverVersion.gitVersion | ltrimstr("v")')"
fi

case "$kubernetes_version" in
  1.14.*)
    kubectl_version="1.16"
    ;;
  1.15.*)
    kubectl_version="1.16"
    ;;
  1.16.*)
    kubectl_version="1.16"
    ;;
  1.17.*)
    kubectl_version="1.16"
    ;;
  1.18.*)
    kubectl_version="1.19"
    ;;
  1.19.*)
    kubectl_version="1.19"
    ;;
  *)
    >&2 echo "ERROR: unsupported kubernetes version $kubernetes_version"
    exit 1
    ;;
esac

exec "/usr/local/bin/kubectl-$kubectl_version" "$@"
