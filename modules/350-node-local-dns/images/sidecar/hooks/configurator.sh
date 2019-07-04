#!/usr/bin/env bash

set -Eeo pipefail

if [[ $1 == "--config" ]] ; then
  cat << EOF
{
   "onStartup": 10,
   "onKubernetesEvent": [
      {
        "kind": "Endpoints",
        "event": ["add", "update"],
        "objectName": "kube-dns",
        "namespaceSelector": {
          "matchNames": [
              "kube-system"
          ]
        }
      }
   ]
}
EOF
else
  /config.sh > /etc/coredns/Corefile
fi
